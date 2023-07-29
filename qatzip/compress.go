// Copyright(c) 2022-2023 Intel Corporation. All rights reserved.

// Package qatzip implements Go bindings for the Intel® Quick Assist Technology compression library
package qatzip

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"runtime/trace"
	"strconv"
	"time"

	"github.com/DataDog/zstd"
)

// Writer implements an io.Writer. When written to, it sends compressed content to w.
type Writer struct {
	w            io.Writer
	closed       bool
	wroteHeader  bool
	err          error
	q            *QzBinding // internal QAT state
	bounceBuf    []byte
	outputBuf    *bytes.Buffer
	bufferGrowth int
	p            params
	ctx          context.Context // context for tracing
	task         *trace.Task     // task for tracing
	perf         *Perf           // perfomance counters
}

const (
	/* Gzip Header magic numbers */
	gzipID1       uint8 = 0x1f
	gzipID2       uint8 = 0x8b
	gzipDeflate   uint8 = 0x08
	osType        uint8 = 255 /* unknown OS type */
	deflateMagic1 uint8 = 0x01
	deflateMagic2 uint8 = 0xff
)

const (
	/* LZ4 header magic numbers */
	lz4ID     uint32 = 0x184D2204
	lz4FLG    uint8  = 0x64
	lz4BD     uint8  = 0x40
	lz4HC     uint8  = 0xa7
	lz4Magic1 uint8  = 0x05
	lz4Magic2 uint8  = 0x5d
	lz4Magic3 uint8  = 0xcc
	lz4Magic4 uint8  = 0x02
)

const (
	envAlgorithm      = "QATGO_ALGORITHM"
	envCompressionLvl = "QATGO_COMPRESSION_LEVEL"
)

// Performance counters
type Perf struct {
	ReadTimeNS   uint64 // time (ns) spent reading from r
	WriteTimeNS  uint64 // time (ns) spent writing to w
	BytesIn      uint64 // bytes sent to QATzip
	BytesOut     uint64 // bytes received from QATzip
	EngineTimeNS uint64 // time (ns) spent in QATzip
	CopyTimeNS   uint64 // time (ns) spent copying buffers + reallocation
}

func applyEnvOptions(z *Writer) (err error) {
	algorithmStr := os.Getenv(envAlgorithm)
	if algorithmStr != "" {
		algorithm, ok := algorithmConv[algorithmStr]
		if !ok {
			err = ErrParamAlgorithm
			return err
		}
		if err := z.Apply(AlgorithmOption(algorithm)); err != nil {
			return err
		}
	}
	compLvlStr := os.Getenv(envCompressionLvl)
	if compLvlStr != "" {
		var compLvl int
		if compLvl, err = strconv.Atoi(compLvlStr); err != nil {
			err = ErrParamCompressionLevel
			return err
		}
		if err := z.Apply(CompressionLevelOption(compLvl)); err != nil {
			return err
		}
	}

	return nil
}

// NewWriter creates a new Writer with output io.Writer w
func NewWriter(w io.Writer) *Writer {
	z := new(Writer)
	z.closed = true
	z.p = defaultParams()
	z.err = applyEnvOptions(z)
	z.w = w
	return z
}

// NewWriterLevel creates a new Writer with an additional compression level setting
func NewWriterLevel(w io.Writer, level int) (z *Writer, err error) {
	z = NewWriter(w)
	err = z.Apply(CompressionLevelOption(level))
	return
}

// Writes an empty header and footer for gzip and lz4
// This is a workaround due to QATzip not supporting empty files
func (z *Writer) writeEmptyBuffer() (err error) {
	var buf []byte
	var le = binary.LittleEndian
	switch z.p.Algorithm {
	case DEFLATE:
		hdr := [10]byte{0: gzipID1, 1: gzipID2, 2: gzipDeflate, 8: byte(z.p.Level), 9: osType}
		magic := [5]byte{0: deflateMagic1, 3: deflateMagic2, 4: deflateMagic2}
		ftr := [8]byte{}
		buf = append(hdr[:], magic[:]...)
		buf = append(buf, ftr[:]...)
	case LZ4:
		hdr := [4]byte{}
		le.PutUint32(hdr[:4], lz4ID)
		frm := [3]byte{0: lz4FLG, 1: lz4BD, 2: lz4HC}
		end := [4]byte{}
		magic := [4]byte{0: lz4Magic1, 1: lz4Magic2, 2: lz4Magic3, 3: lz4Magic4}
		buf = append(hdr[:], frm[:]...)
		buf = append(buf, end[:]...)
		buf = append(buf, magic[:]...)
	case ZSTD:
		_, err := zstd.Compress(buf, buf)
		if err != nil {
			err = ErrFail
			return err
		}
	default:
		err = ErrUnsupportedFmt
		return err
	}
	z.traceLogf(Med, "[write->output] header/footer")
	_, err = z.w.Write(buf)
	z.wroteHeader = true
	return err
}

// Close closes Writer and flushes remaining data
func (z *Writer) Close() (err error) {
	if z.closed {
		z.err = ErrWriterClosed
		return
	}

	z.traceLogf(Med, "[close] err:'%v'", z.err)

	if z.err != nil {
		return z.err
	}

	defer z.task.End()

	if !z.wroteHeader && z.perf.BytesIn == 0 && len(z.bounceBuf) == 0 {
		r := trace.StartRegion(z.ctx, "Qz(5) Empty Buffer")
		z.err = z.writeEmptyBuffer()
		r.End()
		if z.err != nil {
			return z.err
		}
	}

	z.closed = true

	r := trace.StartRegion(z.ctx, "Qz(4) Last Write")
	defer r.End()

	if z.err == nil {
		z.q.SetLast(true)
		err := z.flushBounceBuffer()
		if err != nil {
			z.q.Close()
			return z.err
		}
	}

	z.err = z.q.Close()

	if z.err != nil {
		return z.err
	}

	return
}

// Reset discards current state, loads applied options, and restarts session
func (z *Writer) Reset(w io.Writer) (err error) {
	z.Close()
	z.ctx, z.task = trace.NewTask(context.Background(), "Qz io.Writer")

	if z.p.DebugLevel == None {
		z.p.DebugLevel = getTraceLevel()
	}

	z.q, err = NewQzBinding()
	if err != nil {
		z.err = err
		return
	}
	z.q.setParams(z.p)
	if err = z.q.StartSession(); err != nil {
		z.err = err
		return
	}

	z.outputBuf = bytes.NewBuffer(make([]byte, z.p.OutputBufLength))

	z.w = w
	z.closed = false
	z.err = nil
	z.wroteHeader = false
	z.bufferGrowth = z.p.BufferGrowth
	z.bounceBuf = make([]byte, 0, z.p.BounceBufferLength)
	z.perf = new(Perf)

	return
}

// Write() inputs data from p and writes compressed output data io.Writer w
func (z *Writer) Write(p []byte) (n int, err error) {
	if z.err != nil {
		return 0, z.err
	}

	if z.q == nil {
		if z.err = z.Reset(z.w); z.err != nil {
			return 0, z.err
		}
	}

	r := trace.StartRegion(z.ctx, "Qz(1) Write()")
	defer r.End()

	b := p
	nw := 0
	nb := 0

	err = z.flushBounceBuffer()
	if err != nil {
		z.err = err
		return 0, err
	}

	if z.p.InputBufferMode != NoLast {
		if z.p.InputBufferMode == Bounce || len(p) <= z.p.BounceBufferLength {
			t1 := time.Now().UnixNano()
			if len(p) > len(z.bounceBuf) {
				z.bounceBuf = make([]byte, 0, len(p))
			}

			z.bounceBuf = z.bounceBuf[:len(p)]

			n = copy(z.bounceBuf, p)
			t2 := time.Now().UnixNano()
			z.perf.CopyTimeNS += uint64(t2 - t1)
			return
		}

		if z.p.InputBufferMode == Reserve {
			t1 := time.Now().UnixNano()
			ibl := len(p) - z.p.BounceBufferLength
			b = p[:ibl]
			rb := p[ibl:]
			z.bounceBuf = z.bounceBuf[:len(rb)]
			nb = copy(z.bounceBuf, rb)
			t2 := time.Now().UnixNano()
			z.perf.CopyTimeNS += uint64(t2 - t1)
		}
	}

	nw, err = z.compressWrite(b)
	n = nw + nb
	if err != nil {
		z.err = err
		return n, err
	}

	return
}

func (z *Writer) flushBounceBuffer() (err error) {
	if len(z.bounceBuf) > 0 {
		nw, err := z.compressWrite(z.bounceBuf)
		if err != nil {
			z.err = err
			return err
		}
		if nw < len(z.bounceBuf) {
			z.err = io.ErrShortWrite
			return z.err
		}
	}

	z.bounceBuf = z.bounceBuf[:0]
	return
}

// Compresses transfer buffer
func (z *Writer) compressWrite(p []byte) (n int, err error) {
	var t1, t2 int64 // for performance counters

	if z.err != nil {
		return 0, z.err
	}

	remainder := len(p) // bytes requested from input
	produced := 0       // data copied into p[]
	consumed := 0

	for remainder > 0 {
		if z.p.InputBufferMode == Last {
			z.q.SetLast(true)
		}
		// compress input data
		r := trace.StartRegion(z.ctx, "Qz(2) Compress")
		t1 = time.Now().UnixNano()
		in, out, err := z.q.Compress(p[consumed:], z.outputBuf.Bytes())
		if err == nil {
			z.perf.BytesIn += uint64(in)
			z.perf.BytesOut += uint64(out)
			t2 = time.Now().UnixNano()
			z.perf.EngineTimeNS += uint64(t2 - t1)
		}
		r.End()

		z.traceLogf(Med, "[write->qat] r:%v i:%v o:%v ibofs:%v obl:%v err:%v", remainder, in, out, consumed, z.outputBuf.Len(), err)

		if err != nil {
			if err == ErrBuffer {
				// expand output buffer
				// TODO grow to a maximum size
				t1 = time.Now().UnixNano()
				z.bufferGrowth *= 2
				newSize := remainder + z.bufferGrowth
				z.traceLogf(Med, "[expand output buffer] o:%v n:%v", z.outputBuf.Len(), newSize)
				z.outputBuf = bytes.NewBuffer(make([]byte, newSize))
				t2 = time.Now().UnixNano()
				z.perf.CopyTimeNS += uint64(t2 - t1)
				continue
			}
			z.err = err
			return consumed, err
		}
		z.wroteHeader = true
		consumed += in
		remainder -= in
		produced = out

		if produced > 0 {
			r := trace.StartRegion(z.ctx, "Qz(3) Output Stream")
			t1 = time.Now().UnixNano()
			nw, err := z.w.Write(z.outputBuf.Bytes()[:produced])
			t2 = time.Now().UnixNano()
			z.perf.WriteTimeNS += uint64(t2 - t1)
			r.End()

			z.traceLogf(Med, "[write->output] nw:%v err:%v", nw, err)

			if err != nil {
				z.err = err
				return consumed, err
			}
		}
	}

	return consumed, nil
}

// Get performance counters from Writer
func (z *Writer) GetPerf() Perf {
	return *z.perf
}

// Apply options to Writer
func (z *Writer) Apply(options ...Option) (err error) {
	if z.q != nil {
		err = ErrApplyPostInit
		return
	}

	for _, op := range options {
		if err = op(z); err != nil {
			return
		}
	}
	return
}
