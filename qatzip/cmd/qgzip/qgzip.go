// Copyright(c) 2022-2023 Intel Corporation. All rights reserved.

/*
qgzip is a compression utility implemented with QATgo library

Usage:

	qgzip [flags] [name ...]

The flags are:

	  -A string
	        algorithm (default "gzip")
			"gzip" QATzip DEFLATE/gzip
			"raw" QATzip DEFLATE/raw
			"lz4" QATzip lz4 (QAT 2.0)
			"zstd" QAT zstd plugin (QAT 2.0)
			"sw_gzip" stdlib compress/gzip
			"sw_lz4" pierre/lz4
			"sw_raw" stdlib compress/flate
			"sw_zstd" DataDog/zstd

	  -c    output to stdout
	  -csv
	        show performance stats in CSV
	  -d    decompress if set otherwise compress
	  -debug int
	        enable debug output (1-4)
	  -f    force
	  -h    help
	  -ibm int
	        input buffer mode setting (for testing)
	  -ibs int
	        input buffer size (default 134217728)
	  -k    do not delete input file
	  -l int
	        compression level (default 1)
	  -loop int
	        repeat n times (default 1)
	  -obs int
	        output buffer size (default 134217728)
	  -p    parallel execution
	  -s    show performance stats
	  -t    test decompression of file
	  -trace file
	        write runtime trace to file
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime/trace"
	"sync"
	"syscall"
	"unsafe"

	"github.com/intel/qatgo/qatzip"
)

const (
	fOutputFlags = os.O_RDWR | os.O_TRUNC | os.O_CREATE
	fInputFlags  = os.O_RDONLY
	fTestFlags   = os.O_RDWR
)

const (
	defaultBufLength = 128 * 1024 * 1024
)

const (
	algorithmGzip         = "gzip"
	algorithmLZ4          = "lz4"
	algorithmZstd         = "zstd"
	algorithmRawDeflate   = "raw"
	algorithmSWGzip       = "sw_gzip"
	algorithmSWLZ4        = "sw_lz4"
	algorithmSWRawDeflate = "sw_raw"
	algorithmSWZstd       = "sw_zstd"
)

var (
	algorithm     = flag.String("A", algorithmGzip, "algorithm")
	pipeOut       = flag.Bool("c", false, "output to Stdout")
	level         = flag.Int("l", 1, "compression level")
	traceFile     = flag.String("trace", "", "write go runtime trace to file")
	debug         = flag.Int("debug", 0, "enable debug output")
	decompress    = flag.Bool("d", false, "decompress if set otherwise compress")
	keep          = flag.Bool("k", false, "do not delete input file")
	force         = flag.Bool("f", false, "overwrite safety checks")
	help          = flag.Bool("h", false, "display help")
	parallel      = flag.Bool("p", false, "run operations in parallel")
	inputBufSize  = flag.Int("ibs", defaultBufLength, "input buffer size")
	outputBufSize = flag.Int("obs", defaultBufLength, "output buffer size")
	showStats     = flag.Bool("s", false, "show performance stats")
	showStatsCSV  = flag.Bool("csv", false, "show performance stats in CSV")
	loops         = flag.Int("loop", 1, "repeat command n times")
	inputBufMode  = flag.Int("ibm", 0, "input buffer mode setting")
	test          = flag.Bool("t", false, "test decompression of file")
)

var wg sync.WaitGroup
var errExitCode int

func suggestHelp() {
	fmt.Fprintln(os.Stderr, "for help, type:", os.Args[0], "-h")
}

func isatty(f *os.File) bool {
	var p uint64
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&p)))
	return err != syscall.ENOTTY
}

func main() {
	defer func() {
		os.Exit(errExitCode)
	}()

	flag.Parse()

	args := flag.Args()

	var fileList []string
	argStdin := false

	for _, a := range args {
		if a == "-" {
			if !argStdin {
				fileList = append(fileList, "-")
			}
			argStdin = true
			continue
		}

		fileList = append(fileList, a)
	}

	if *help {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "QATgo Compression Utility")
		flag.PrintDefaults()
		return
	}

	if *test {
		*decompress = true
	}

	if *traceFile != "" {
		f, err := os.Create(*traceFile)
		if err != nil {
			log.Fatalf("error: failed to create trace file %v: %v", *traceFile, err)
		}
		defer f.Close()
		if err := trace.Start(f); err != nil {
			log.Fatalf("error: failed to start trace: %v", err)
		}
		defer trace.Stop()
	}

	if *inputBufSize <= 0 || *outputBufSize <= 0 {
		log.Fatalf("error: invalid buffersize ibs=%v obs=%v", *inputBufSize, *outputBufSize)
	}

	if len(fileList) == 0 {
		*pipeOut = true
	}

	// this mimics gzip behavior
	if !*decompress && *pipeOut && isatty(os.Stdout) && !*force {
		fmt.Fprintln(os.Stderr, "error: won't output binary data to terminal.")
		suggestHelp()
		errExitCode = 1
		return
	}

	nch := 1
	if len(fileList) > 0 {
		nch = len(fileList)
	}

	errch := make(chan error, nch)
	workch := make(chan *workItem, *loops*nch)
	printHeader := true

	for j := 0; j < *loops; j++ {
		if len(fileList) > 0 {
			for i, fileName := range fileList {
				w := new(workItem)
				w.jobId = i
				w.loopCnt = j
				w.fileName = fileName
				w.errch = errch
				w.workch = workch
				wg.Add(1)
				if *parallel {
					go doWork(w)
				} else {
					doWork(w)
				}
			}
		} else {
			wg.Add(1)
			w := new(workItem)
			w.jobId = 0
			w.loopCnt = j
			w.fileName = ""
			doWork(w)
		}

		wg.Wait()

		for done := false; !done; {
			select {
			case e := <-errch:
				errExitCode = 1
				fmt.Fprintf(os.Stderr, "%v\n", e)
			default:
				done = true
			}
		}

		if *showStats || *showStatsCSV {
			if *showStatsCSV && printHeader {
				printCSVHeader()
				printHeader = false
			}

			for done := false; !done; {
				select {
				case w := <-workch:
					if *showStatsCSV {
						dumpStatsCSV(w)
					} else {
						dumpStats(w)
					}
				default:
					done = true
				}
			}
		}
	}
	close(errch)
}

func doWork(w *workItem) {

	defer wg.Done()
	var fin, fout, ftest *os.File
	var err error
	var suffix string
	name := w.fileName

	if w.fileName == "" || w.fileName == "-" {
		fin = os.Stdin
		fout = os.Stdout
		name = "-"
	}

	// QAT settings
	w.alg = qatzip.DEFLATE
	w.dfmt = qatzip.Deflate48

	switch *algorithm {
	case algorithmGzip:
		w.alg = qatzip.DEFLATE
		w.dfmt = qatzip.DeflateGzipExt
		fallthrough
	case algorithmSWGzip:
		suffix = ".gz"

	case algorithmLZ4:
		w.alg = qatzip.LZ4
		fallthrough
	case algorithmSWLZ4:
		suffix = ".lz4"

	case algorithmZstd:
		w.alg = qatzip.ZSTD
		fallthrough
	case algorithmSWZstd:
		suffix = ".zst"

	case algorithmRawDeflate:
		w.alg = qatzip.DEFLATE
		w.dfmt = qatzip.DeflateRaw
		fallthrough
	case algorithmSWRawDeflate:
		suffix = ".bin"
	default:
		w.errch <- fmt.Errorf("%s: error: algorithm not supported", name)
		return
	}

	if !*decompress && fin != os.Stdin {
		fin, err = os.OpenFile(w.fileName, fInputFlags, 0755)
		if err != nil {
			w.errch <- fmt.Errorf("%s: %v", name, err)
			return
		}

		if !*pipeOut {
			// check if the file exists
			ftest, err = os.OpenFile(w.fileName+suffix, fInputFlags, 0755)
			if err == nil && !*force {
				w.errch <- fmt.Errorf("%s: error: file %s already exists; not overwritten", name, w.fileName+suffix)
				return
			} else {
				ftest.Close()
			}

			fout, err = os.OpenFile(w.fileName+suffix, fOutputFlags, 0664)
			if err != nil {
				w.errch <- fmt.Errorf("%s: %v", name, err)
				return
			}
		} else {
			fout = os.Stdout
		}
	}

	if *decompress && fin != os.Stdin {
		fin, err = os.OpenFile(w.fileName, fInputFlags, 0755)
		if err != nil {
			w.errch <- fmt.Errorf("%s: %v", name, err)
			return
		}

		if *test {
			fout, err = os.OpenFile("/dev/null", fTestFlags, 0755)
			if err != nil {
				w.errch <- fmt.Errorf("%s: %v", name, err)
				return
			}
		} else if !*pipeOut {
			// validate suffix
			re := regexp.MustCompile(suffix + "$")
			i := re.FindStringIndex(w.fileName)
			if i == nil {
				w.errch <- fmt.Errorf("%s: error: file %s invalid suffix (expected %s) -- ignored", name, w.fileName, suffix)
				return
			}
			ofn := w.fileName[:i[0]]

			// check if the file exists
			ftest, err = os.OpenFile(ofn, fInputFlags, 0755)
			if err == nil && !*force {
				w.errch <- fmt.Errorf("%s: error: file %s already exists; not overwritten", name, ofn)
				return
			} else {
				ftest.Close()
			}

			fout, err = os.OpenFile(ofn, fOutputFlags, 0664)
			if err != nil {
				w.errch <- fmt.Errorf("%s: %v", name, err)
				return
			}
		} else {
			fout = os.Stdout
		}
	}

	if fout == os.Stdout && *parallel {
		w.errch <- fmt.Errorf("%s: error: file %s cannot output to stdout in parallel mode", name, w.fileName)
		return
	}

	if !*decompress {
		switch *algorithm {
		case algorithmGzip:
			fallthrough
		case algorithmRawDeflate:
			fallthrough
		case algorithmZstd:
			fallthrough
		case algorithmLZ4:
			err = compressQAT(fin, fout, w)

		case algorithmSWGzip:
			err = compressSWGzip(fin, fout, *level)
		case algorithmSWLZ4:
			err = compressSWLz4(fin, fout, *level)
		case algorithmSWRawDeflate:
			err = compressSWRaw(fin, fout, *level)
		case algorithmSWZstd:
			err = compressSWZstd(fin, fout, *level)
		default:
			w.errch <- fmt.Errorf("%s: error: algorithm not supported", name)
			return
		}
	} else {
		switch *algorithm {
		case algorithmGzip:
			fallthrough
		case algorithmRawDeflate:
			fallthrough
		case algorithmZstd:
			fallthrough
		case algorithmLZ4:
			err = decompressQAT(fin, fout, w)
		case algorithmSWGzip:
			err = decompressSWGzip(fin, fout)
		case algorithmSWLZ4:
			err = decompressSWLz4(fin, fout)
		case algorithmSWRawDeflate:
			err = decompressSWRaw(fin, fout)
		case algorithmSWZstd:
			err = decompressSWZstd(fin, fout)
		default:
			w.errch <- fmt.Errorf("%s: error: algorithm not supported", name)
			return
		}
	}

	if err != nil {
		w.errch <- fmt.Errorf("%s: %v", name, err)
		return
	}

	if fin != os.Stdin {
		fin.Close()
	}
	if fout != os.Stdout {
		fout.Close()
	}

	if !*test && !*keep && !*pipeOut && err == nil && fout != os.Stdout {
		err := os.Remove(w.fileName)
		if err != nil {
			w.errch <- fmt.Errorf("%s: error: removing file; err: %v", w.fileName, err)
			return
		}
	}

	// send completed work item back to main()
	w.workch <- w
}
