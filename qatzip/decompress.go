// Copyright(c) 2022-2023 Intel Corporation. All rights reserved.

package qatzip

import (
	"context"
	"fmt"
	"io"
	"runtime/trace"
	"time"
)

// Reader implements an io.Reader. When read from, it decompresses content from r.
type Reader struct {
	r               io.Reader
	closed          bool
	err             error
	q               *QzBinding // internal QAT state
	inputBuf        []byte
	inputBufOffset  int
	inputBufRead    int
	outputBuf       []byte
	outputBufOffset int
	outputBufLeft   int
	streamDone      bool
	bufferGrowth    int
	p               params
	ctx             context.Context // context for tracing
	task            *trace.Task     // task for tracing
	perf            *Perf           // performance counters
}

// NewReader creates a new Reader with input io.Reader r
func NewReader(r io.Reader) (*Reader, error) {
	z := new(Reader)
	z.closed = true
	z.p = defaultParams()
	z.r = r
	return z, nil
}

// Close closes Reader
func (z *Reader) Close() error {
	if z.closed {
		return z.err
	}

	z.traceLogf(Med, "[close] err:'%v'", z.err)

	if z.err != nil {
		return z.err
	}

	defer z.task.End()

	z.closed = true

	if z.q == nil {
		z.err = ErrNone
		return z.err
	}

	z.err = z.q.Close()

	return z.err
}

// Reset discards current state, loads applied options, and restarts session
func (z *Reader) Reset(r io.Reader) error {
	z.err = z.Close()
	if z.err != nil {
		return z.err
	}

	if z.p.DebugLevel == None {
		z.p.DebugLevel = getTraceLevel()
	}

	z.ctx, z.task = trace.NewTask(context.Background(), "Qz io.Reader")
	z.q, z.err = NewQzBinding()
	if z.q == nil {
		return z.err
	}
	z.q.setParams(z.p)
	if z.err = z.q.StartSession(); z.err != nil {
		return z.err
	}

	z.streamDone = false
	z.inputBufRead = 0
	z.bufferGrowth = z.p.BufferGrowth

	z.r = r

	z.inputBuf = make([]byte, z.p.InputBufLength)
	z.outputBuf = make([]byte, z.p.OutputBufLength)

	z.inputBufOffset = 0
	z.outputBufOffset = 0
	z.outputBufLeft = 0

	z.closed = false

	z.perf = new(Perf)

	return nil
}

// Read() reads compressed data from io.Reader r and outputs decompressed data to p.
func (z *Reader) Read(p []byte) (n int, err error) {
	var t1, t2 int64 // for performance counters
	if z.err != nil {
		return 0, z.err
	}

	if z.q == nil {
		if z.err = z.Reset(z.r); z.err != nil {
			return 0, z.err
		}
	}

	r := trace.StartRegion(z.ctx, "Qz(1) Read()")
	defer r.End()

	remainder := len(p) // bytes requested from input stream
	produced := 0       // data copied into p[]

	for remainder > 0 {

		// drain output buffer
		if z.outputBufLeft > 0 {
			t1 = time.Now().UnixNano()
			np := copy(p[produced:], z.outputBuf[z.outputBufOffset:z.outputBufOffset+z.outputBufLeft])
			t2 = time.Now().UnixNano()
			z.perf.CopyTimeNS += uint64(t2 - t1)
			z.outputBufOffset += np
			z.outputBufLeft -= np
			produced += np
			remainder -= np
			continue
		}

		if z.inputBufRead-z.inputBufOffset < 0 {
			z.err = fmt.Errorf(QatErrHdr+"internal assert: ibl:%v < ibofs:%v", z.inputBufRead, z.inputBufOffset)
			return 0, z.err
		}
		// fetch compressed data from input stream
		if z.inputBufRead-z.inputBufOffset == 0 {
			if z.streamDone {
				return produced, io.EOF
			}

			rr := trace.StartRegion(z.ctx, "Qz(3) Input Stream")
			for !z.streamDone {
				t1 = time.Now().UnixNano()
				nt, err := z.r.Read(z.inputBuf[z.inputBufRead:])
				t2 = time.Now().UnixNano()
				z.perf.ReadTimeNS += uint64(t2 - t1)
				z.inputBufRead += nt
				z.traceLogf(Med, "[transfer] nt:%v iblen:%v ibr:%v err:%v", nt, len(z.inputBuf), z.inputBufRead, err)
				if z.inputBufRead >= len(z.inputBuf) {
					t1 = time.Now().UnixNano()
					s := z.inputBufRead * 2
					z.traceLogf(Med, "[expand input buffer] iblen:%v -> %v", len(z.inputBuf), s+len(z.inputBuf))
					b := append(z.inputBuf, make([]byte, s)...)
					t2 = time.Now().UnixNano()
					z.perf.CopyTimeNS += uint64(t2 - t1)
					z.inputBuf = b
				}

				if err != nil {
					if err != io.EOF {
						z.err = err
						return 0, err
					}
					z.streamDone = true
				}
			}
			rr.End()
		}

		// decompress input data
		rq := trace.StartRegion(z.ctx, "Qz(2) Decompress")
		t1 = time.Now().UnixNano()
		in, out, err := z.q.Decompress(z.inputBuf[z.inputBufOffset:z.inputBufRead], z.outputBuf)
		if err == nil {
			z.perf.BytesIn += uint64(in)
			z.perf.BytesOut += uint64(out)
		}
		t2 = time.Now().UnixNano()
		z.perf.EngineTimeNS += uint64(t2 - t1)
		rq.End()

		z.traceLogf(Med, "[read->QAT] i:%v o:%v iblen:%v ibofs:%v ibr:%v obl:%v err:%v",
			in, out, len(z.inputBuf), z.inputBufOffset, z.inputBufRead, len(z.outputBuf), err)

		if err != nil {
			if err == ErrBuffer {
				// expand output buffer
				// TODO grow to a maximum size
				t1 = time.Now().UnixNano()
				z.bufferGrowth *= 2
				z.traceLogf(Med, "[expand output buffer] obl:%v -> %v", len(z.outputBuf), remainder+z.bufferGrowth)
				z.outputBuf = make([]byte, remainder+z.bufferGrowth)
				t2 = time.Now().UnixNano()
				z.perf.CopyTimeNS += uint64(t2 - t1)
				continue
			}
			z.err = err
			return produced, err
		}

		z.inputBufOffset += in
		z.outputBufOffset = 0
		z.outputBufLeft = out
	}

	return produced, nil
}

// Get performance counters from Reader
func (z *Reader) GetPerf() Perf {
	return *z.perf
}

// Apply options to Reader
func (z *Reader) Apply(options ...Option) (err error) {
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
