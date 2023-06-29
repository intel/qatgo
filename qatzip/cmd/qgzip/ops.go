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

func compressQAT(fin *os.File, fout *os.File, alg qatzip.Algorithm, dfmt qatzip.DeflateFmt) (err error) {
	r1 := new(syscall.Rusage)
	r2 := new(syscall.Rusage)
	syscall.Getrusage(syscall.RUSAGE_SELF, r1)
	t1 := time.Now().UnixNano()

	z := qatzip.NewWriter(fout)

	err = z.Apply(
		qatzip.CompressionLevelOption(*level),
		qatzip.InputBufferModeOption(qatzip.InputBufferMode(*inputBufMode)),
		qatzip.OutputBufLengthOption(*outputBufSize),
		qatzip.AlgorithmOption(alg),
		qatzip.DeflateFmtOption(dfmt),
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

	if *showStats && err == nil {
		fmt.Fprintf(os.Stderr, "Wall Time %d ms\n", (t3-t1)/1_000_000)
		fmt.Fprintf(os.Stderr, "User CPU Time %d ms\n", (r2.Utime.Nano()-r1.Utime.Nano())/1_000_000)
		fmt.Fprintf(os.Stderr, "System CPU Time %d ms\n", (r2.Stime.Nano()-r1.Stime.Nano())/1_000_000)
		fmt.Fprintf(os.Stderr, "Init Time %d ms\n", (t2-t1)/1_000_000)
		dumpStats(z.GetPerf(), true)
	}

	if *showStatsCSV && err == nil {
		fmt.Fprintf(os.Stderr, "c,")
		fmt.Fprintf(os.Stderr, "%d,", (t3-t1)/1_000_000)                           // Wall Time
		fmt.Fprintf(os.Stderr, "%d,", (r2.Utime.Nano()-r1.Utime.Nano())/1_000_000) // User CPU
		fmt.Fprintf(os.Stderr, "%d,", (r2.Stime.Nano()-r1.Stime.Nano())/1_000_000) // System CPU
		fmt.Fprintf(os.Stderr, "%d,", (t2-t1)/1_000_000)                           // Init Time
		dumpStatsCSV(z.GetPerf(), true)
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

func decompressQAT(fin *os.File, fout *os.File, alg qatzip.Algorithm, dfmt qatzip.DeflateFmt) (err error) {
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
		qatzip.AlgorithmOption(alg),
		qatzip.DeflateFmtOption(dfmt),
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

	if *showStats && err == nil {
		fmt.Fprintf(os.Stderr, "Wall Time %d ms\n", (t3-t1)/1_000_000)
		fmt.Fprintf(os.Stderr, "User CPU Time %d ms\n", (r2.Utime.Nano()-r1.Utime.Nano())/1_000_000)
		fmt.Fprintf(os.Stderr, "System CPU Time %d ms\n", (r2.Stime.Nano()-r1.Stime.Nano())/1_000_000)
		fmt.Fprintf(os.Stderr, "Init Time %d ms\n", (t2-t1)/1_000_000)
		dumpStats(z.GetPerf(), false)
	}

	if *showStatsCSV && err == nil {
		fmt.Fprintf(os.Stderr, "d,")
		fmt.Fprintf(os.Stderr, "%d,", (t3-t1)/1_000_000)                           // Wall Time
		fmt.Fprintf(os.Stderr, "%d,", (r2.Utime.Nano()-r1.Utime.Nano())/1_000_000) // User CPU
		fmt.Fprintf(os.Stderr, "%d,", (r2.Stime.Nano()-r1.Stime.Nano())/1_000_000) // System CPU
		fmt.Fprintf(os.Stderr, "%d,", (t2-t1)/1_000_000)                           // Init Time
		dumpStatsCSV(z.GetPerf(), false)
	}

	return err
}

func dumpStats(perf qatzip.Perf, compress bool) {
	fmt.Fprintf(os.Stderr, "Bytes In %d\n", perf.BytesIn)
	fmt.Fprintf(os.Stderr, "Bytes Out %d\n", perf.BytesOut)
	fmt.Fprintf(os.Stderr, "Read Time  %d ms\n", perf.ReadTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "Write Time  %d ms\n", perf.WriteTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "Engine Time %d ms\n", perf.EngineTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "Copy Time %d ms\n", perf.CopyTimeNS/1_000_000)

	if compress {
		fmt.Fprintf(os.Stderr, "Compression Ratio %f\n", float64(perf.BytesIn)/float64(perf.BytesOut))
		fmt.Fprintf(os.Stderr, "Compression Speed %f MB/s (Engine)\n", (float64(perf.BytesIn)/1_000_000.0)/(float64(perf.EngineTimeNS)/1_000_000_000.0))
	} else {
		fmt.Fprintf(os.Stderr, "Compression Ratio %f\n", float64(perf.BytesOut)/float64(perf.BytesIn))
		fmt.Fprintf(os.Stderr, "Compression Speed %f MB/s (Engine)\n", (float64(perf.BytesOut)/1_000_000.0)/(float64(perf.EngineTimeNS)/1_000_000_000.0))
	}

}

func dumpStatsCSV(perf qatzip.Perf, compress bool) {
	fmt.Fprintf(os.Stderr, "%d,", perf.BytesIn)
	fmt.Fprintf(os.Stderr, "%d,", perf.BytesOut)
	fmt.Fprintf(os.Stderr, "%d,", perf.ReadTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "%d,", perf.WriteTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "%d,", perf.EngineTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "%d,", perf.CopyTimeNS/1_000_000)

	if compress {
		fmt.Fprintf(os.Stderr, "%f,", float64(perf.BytesIn)/float64(perf.BytesOut))
		fmt.Fprintf(os.Stderr, "%f\n", (float64(perf.BytesIn)/1_000_000.0)/(float64(perf.EngineTimeNS)/1_000_000_000.0))
	} else {
		fmt.Fprintf(os.Stderr, "%f,", float64(perf.BytesOut)/float64(perf.BytesIn))
		fmt.Fprintf(os.Stderr, "%f\n", (float64(perf.BytesOut)/1_000_000.0)/(float64(perf.EngineTimeNS)/1_000_000_000.0))
	}

}
