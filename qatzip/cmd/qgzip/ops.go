// Copyright(c) 2022-2023 Intel Corporation. All rights reserved.
package main

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/DataDog/zstd"
	"github.com/intel/qatgo/qatzip"
	"github.com/pierrec/lz4/v4"
)

type workItem struct {
	fileName string // File Name
	jobId    int    // Job Id
	loopCnt  int    // Loop Counter
	alg      qatzip.Algorithm
	dfmt     qatzip.DeflateFmt
	perf     perf
	errch    chan<- error
	workch   chan<- *workItem
}

func compressSWGzip(fin *os.File, fout *os.File, level int) (err error) {
	g, err := gzip.NewWriterLevel(fout, level)
	if err != nil {
		return err
	}
	_, err = io.Copy(g, fin)
	g.Close()
	return err
}

func compressSWLz4(fin *os.File, fout *os.File, level int) (err error) {
	zw := lz4.NewWriter(fout)

	convLevel := map[int]lz4.CompressionLevel{
		0: lz4.Fast,
		1: lz4.Level1,
		2: lz4.Level2,
		3: lz4.Level3,
		4: lz4.Level4,
		5: lz4.Level5,
		6: lz4.Level6,
		7: lz4.Level7,
		8: lz4.Level8,
		9: lz4.Level9,
	}

	if v, ok := convLevel[level]; ok {
		zw.Apply(lz4.CompressionLevelOption(lz4.CompressionLevel(v)))
	} else {
		err = fmt.Errorf("error: invalid lz4 level %d; valid range [0-9]", level)
		return err
	}
	_, err = io.Copy(zw, fin)

	zw.Close()
	return err
}

func compressSWRaw(fin *os.File, fout *os.File, level int) (err error) {
	zw, err := flate.NewWriter(fout, level)
	if err != nil {
		return err
	}

	_, err = io.Copy(zw, fin)

	zw.Close()
	return err
}

func compressSWZstd(fin *os.File, fout *os.File, level int) (err error) {
	zw := zstd.NewWriterLevel(fout, level)

	_, err = io.Copy(zw, fin)

	zw.Close()
	return err
}

func compressQAT(fin *os.File, fout *os.File, w *workItem) (err error) {
	r1 := new(syscall.Rusage)
	r2 := new(syscall.Rusage)
	syscall.Getrusage(syscall.RUSAGE_SELF, r1)
	t1 := time.Now().UnixNano()

	z := qatzip.NewWriter(fout)

	err = z.Apply(
		qatzip.CompressionLevelOption(*level),
		qatzip.InputBufferModeOption(qatzip.InputBufferMode(*inputBufMode)),
		qatzip.OutputBufLengthOption(*outputBufSize),
		qatzip.AlgorithmOption(w.alg),
		qatzip.DeflateFmtOption(w.dfmt),
		qatzip.DebugLevelOption(qatzip.DebugLevel(*debug)),
	)

	if err != nil {
		return err
	}

	t2 := time.Now().UnixNano()

	b := make([]byte, *inputBufSize)
	// perform compression
	_, err = io.CopyBuffer(z, fin, b)

	z.Close()

	t3 := time.Now().UnixNano()
	syscall.Getrusage(syscall.RUSAGE_SELF, r2)

	if err == nil {
		w.perf.wallTimeNS = t3 - t1
		w.perf.userTimeNS = r2.Utime.Nano() - r1.Utime.Nano()
		w.perf.systemTimeNS = r2.Stime.Nano() - r1.Stime.Nano()
		w.perf.initTimeNS = (t2 - t1)
		w.perf.qp = z.GetPerf()
	}

	return err
}

func decompressSWGzip(fin *os.File, fout *os.File) (err error) {
	g, err := gzip.NewReader(fin)
	if err != nil {
		return err
	}
	_, err = io.Copy(fout, g)

	g.Close()
	return err
}

func decompressSWLz4(fin *os.File, fout *os.File) (err error) {
	zr := lz4.NewReader(fin)
	_, err = io.Copy(fout, zr)
	return err
}

func decompressSWRaw(fin *os.File, fout *os.File) (err error) {
	zr := flate.NewReader(fin)
	_, err = io.Copy(fout, zr)
	zr.Close()

	return err
}

func decompressSWZstd(fin *os.File, fout *os.File) (err error) {
	zr := zstd.NewReader(fin)
	_, err = io.Copy(fout, zr)
	zr.Close()

	return err
}

func decompressQAT(fin *os.File, fout *os.File, w *workItem) (err error) {
	r1 := new(syscall.Rusage)
	r2 := new(syscall.Rusage)
	syscall.Getrusage(syscall.RUSAGE_SELF, r1)
	t1 := time.Now().UnixNano()

	z, err := qatzip.NewReader(fin)
	if err != nil {
		return err
	}

	err = z.Apply(
		qatzip.InputBufLengthOption(*inputBufSize),
		qatzip.OutputBufLengthOption(*outputBufSize),
		qatzip.AlgorithmOption(w.alg),
		qatzip.DeflateFmtOption(w.dfmt),
		qatzip.DebugLevelOption(qatzip.DebugLevel(*debug)),
	)

	if err != nil {
		return err
	}

	t2 := time.Now().UnixNano()

	// perform decompression
	_, err = io.Copy(fout, z)
	z.Close()

	t3 := time.Now().UnixNano()
	syscall.Getrusage(syscall.RUSAGE_SELF, r2)

	if err == nil {
		w.perf.wallTimeNS = t3 - t1
		w.perf.userTimeNS = r2.Utime.Nano() - r1.Utime.Nano()
		w.perf.systemTimeNS = r2.Stime.Nano() - r1.Stime.Nano()
		w.perf.initTimeNS = (t2 - t1)
		w.perf.qp = z.GetPerf()
	}

	return err
}
