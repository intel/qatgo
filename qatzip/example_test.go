package qatzip_test

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/intel/qatgo/qatzip"
)

func Example_writerReader() {
	// This is an example of compressing and decompressing using QATgo
	// NewWriter/NewReader provides io.Reader and io.Writer implementations compatible with Go standard library
	buf := new(bytes.Buffer) // contains compressed data
	var str = "this string will be compressed and then decompressed"

	zw := qatzip.NewWriter(buf) // start a compression session with buf as output

	err := zw.Apply(
		qatzip.AlgorithmOption(qatzip.DEFLATE),         // set DEFLATE algorithm (lz4 and zstd are supported as well)
		qatzip.DeflateFmtOption(qatzip.DeflateGzipExt), // set format to gzip extended header (for DEFLATE algorithm only)
		qatzip.CompressionLevelOption(6),               // set compression level 6
	)
	if err != nil {
		log.Fatal("error: could not apply compression options:", err)
	}

	// compress input string and output to buf
	if _, err = zw.Write([]byte(str)); err != nil {
		log.Fatal("error: could not compress input:", err)
	}

	// end compression session: flush remaining input buffer and release resources
	if err = zw.Close(); err != nil {
		log.Fatal("error: could not end compression session:", err)
	}

	zr, err := qatzip.NewReader(buf) // start a decompression session with buf as input
	if err != nil {
		log.Fatal("error: could not start decompression session:", err)
	}

	err = zr.Apply(
		qatzip.AlgorithmOption(qatzip.DEFLATE),         // enable DEFLATE
		qatzip.DeflateFmtOption(qatzip.DeflateGzipExt), // enable gzip extended header
	)
	if err != nil {
		log.Fatal("error: could not apply decompression options:", err)
	}

	// decompress input stream and write to stdout
	if _, err = io.Copy(os.Stdout, zr); err != nil {
		log.Fatal("error: could not decompress input:", err)
	}

	// end decompression session: release accelerator resources
	if err = zr.Close(); err != nil {
		log.Fatal("error: could not end decompression session:", err)
	}

	// Output: this string will be compressed and then decompressed
}

func Example_writerReaderDirect() {
	// This is an example of compressing and decompressing using QATgo's direct methods
	// Direct methods provide direct access to low-level QAT APIs without any high-level QATgo buffer management
	// The methods only supports byte slices for I/O
	// The decompressed data buffer in this example is too small requiring the buffer to be resized

	var str = strings.Repeat("string repeats..", 64)
	cbuf := make([]byte, len(str)+512) // contains compressed data
	dbuf := make([]byte, 512)          // contains decompressed data (will need resizing)

	q, err := qatzip.NewQzBinding() // instantiate direct session object
	if err != nil {
		log.Fatal("error: could not instantiate direct compression session:", err)
	}

	err = q.Apply(
		qatzip.AlgorithmOption(qatzip.DEFLATE),         // enable DEFLATE (lz4 and zstd are supported as well)
		qatzip.DeflateFmtOption(qatzip.DeflateGzipExt), // enable gzip extended header (for DEFLATE algorithm only)
		qatzip.DirOption(qatzip.Compress),              // set direction to compress
		qatzip.CompressionLevelOption(6),               // set compression level 6
	)
	if err != nil {
		log.Fatal("error: could not apply compression options:", err)
	}

	// start new compression session
	if err = q.StartSession(); err != nil {
		log.Fatal("error: could not start compression session:", err)
	}

	// Only one buffer is being compressed, so mark this as the last buffer
	q.SetLast(true)

	// compress input string and output to cbuf, nc is bytes output during compression
	_, nc, err := q.Compress([]byte(str), cbuf)
	if err != nil {
		log.Fatal("error: could not compress input data:", err)
	}

	// end compression session and release resources
	if err = q.Close(); err != nil {
		log.Fatal("error: could not end input session:", err)
	}

	q, err = qatzip.NewQzBinding() // instantiate a new direct session object
	if err != nil {
		log.Fatal("error: could not instatiate direct decompression session:", err)
	}

	err = q.Apply(
		qatzip.AlgorithmOption(qatzip.DEFLATE),         // enable DEFLATE (must match compress)
		qatzip.DeflateFmtOption(qatzip.DeflateGzipExt), // enable gzip extended header
		qatzip.DirOption(qatzip.Decompress),            // set direction to decompress
	)
	if err != nil {
		log.Fatal("error: could not apply compression options:", err)
	}

	// start new compression session
	if err = q.StartSession(); err != nil {
		log.Fatal("error: could not start direct decompression session:", err)
	}

	// decompress input stream and write to stdout, nd = bytes output during decompress
	// in this example the size of the output buffer is too small and will need to be grown
	var nd int
	for {
		if _, nd, err = q.Decompress(cbuf[:nc], dbuf); err != nil {
			// if output buffer is not long enough grow output buffer
			// ErrBuffer means there is insufficient buffer space for output
			if err == qatzip.ErrBuffer {
				dbuf = make([]byte, len(dbuf)*2) // double buffer size and try again
				continue
			}

			log.Fatal("error: could not decompress input data:", err)
		}
		// decompression was successful
		break
	}

	// end decompression session and release accelerator resources
	if err = q.Close(); err != nil {
		log.Fatal("error: could not end decompression session:", err)
	}

	// verify that the decompressed output matches the original input
	if bytes.Equal(dbuf[:nd], []byte(str)) {
		fmt.Println("strings match")
	} else {
		fmt.Println("strings do not match")
	}

	// Output: strings match
}
