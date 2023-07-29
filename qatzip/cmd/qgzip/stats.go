package main

import (
	"fmt"
	"os"

	"github.com/intel/qatgo/qatzip"
)

type perf struct {
	wallTimeNS   int64 // Wall Clock Time
	userTimeNS   int64 // Usermode Time
	systemTimeNS int64 // System Time
	initTimeNS   int64 // Init time
	qp           qatzip.Perf
}

func dumpStats(w *workItem) {
	fmt.Fprintf(os.Stderr, "Job # %d\n", w.jobId)

	if *loops > 0 {
		fmt.Fprintf(os.Stderr, "Loop # %d\n", w.loopCnt)
	}

	fmt.Fprintf(os.Stderr, "Algorithm %s\n", *algorithm)
	fmt.Fprintf(os.Stderr, "Level %d\n", *level)
	fmt.Fprintf(os.Stderr, "User CPU Time %d ms\n", w.perf.userTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "Wall Time %d ms\n", w.perf.wallTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "User CPU Time %d ms\n", w.perf.userTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "System CPU Time %d ms\n", w.perf.systemTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "Init Time %d ms\n", w.perf.initTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "Bytes In %d\n", w.perf.qp.BytesIn)
	fmt.Fprintf(os.Stderr, "Bytes Out %d\n", w.perf.qp.BytesOut)
	fmt.Fprintf(os.Stderr, "Read Time  %d ms\n", w.perf.qp.ReadTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "Write Time  %d ms\n", w.perf.qp.WriteTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "Engine Time %d ms\n", w.perf.qp.EngineTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "Copy Time %d ms\n", w.perf.qp.CopyTimeNS/1_000_000)

	if *decompress {
		fmt.Fprintf(os.Stderr, "Compression Ratio %f\n", float64(w.perf.qp.BytesOut)/float64(w.perf.qp.BytesIn))
		fmt.Fprintf(os.Stderr, "Compression Speed %f MB/s (Engine)\n", (float64(w.perf.qp.BytesOut)/1_000_000.0)/(float64(w.perf.qp.EngineTimeNS)/1_000_000_000.0))
	} else {
		fmt.Fprintf(os.Stderr, "Compression Ratio %f\n", float64(w.perf.qp.BytesIn)/float64(w.perf.qp.BytesOut))
		fmt.Fprintf(os.Stderr, "Compression Speed %f MB/s (Engine)\n", (float64(w.perf.qp.BytesIn)/1_000_000.0)/(float64(w.perf.qp.EngineTimeNS)/1_000_000_000.0))
	}

	fmt.Fprint(os.Stderr, "-------------------------------------------\n")
}

func printCSVHeader() {
	fmt.Fprint(os.Stderr, "op,job,loop,algo,level,time,ucputime,scputime,itime,in,out,rtime,wtime,etime,ctime,ratio,speed\n")
}

func dumpStatsCSV(w *workItem) {
	if *decompress {
		fmt.Fprintf(os.Stderr, "d,")
	} else {
		fmt.Fprintf(os.Stderr, "c,")
	}

	fmt.Fprintf(os.Stderr, "%d,", w.jobId)
	fmt.Fprintf(os.Stderr, "%d,", w.loopCnt)
	fmt.Fprintf(os.Stderr, "%s,", *algorithm)
	fmt.Fprintf(os.Stderr, "%d,", *level)
	fmt.Fprintf(os.Stderr, "%d,", w.perf.wallTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "%d,", w.perf.userTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "%d,", w.perf.systemTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "%d,", w.perf.initTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "%d,", w.perf.qp.BytesIn)
	fmt.Fprintf(os.Stderr, "%d,", w.perf.qp.BytesOut)
	fmt.Fprintf(os.Stderr, "%d,", w.perf.qp.ReadTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "%d,", w.perf.qp.WriteTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "%d,", w.perf.qp.EngineTimeNS/1_000_000)
	fmt.Fprintf(os.Stderr, "%d,", w.perf.qp.CopyTimeNS/1_000_000)

	if *decompress {
		fmt.Fprintf(os.Stderr, "%f,", float64(w.perf.qp.BytesOut)/float64(w.perf.qp.BytesIn))
		fmt.Fprintf(os.Stderr, "%f\n", (float64(w.perf.qp.BytesOut)/1_000_000.0)/(float64(w.perf.qp.EngineTimeNS)/1_000_000_000.0))
	} else {
		fmt.Fprintf(os.Stderr, "%f,", float64(w.perf.qp.BytesIn)/float64(w.perf.qp.BytesOut))
		fmt.Fprintf(os.Stderr, "%f\n", (float64(w.perf.qp.BytesIn)/1_000_000.0)/(float64(w.perf.qp.EngineTimeNS)/1_000_000_000.0))
	}
}
