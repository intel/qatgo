// Copyright(c) 2022-2023 Intel Corporation. All rights reserved.

package qatzip

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/DataDog/zstd"
	"github.com/pierrec/lz4/v4"
)

var strGettysBurgAddress = "" +
	"  Four score and seven years ago our fathers brought forth on\n" +
	"this continent, a new nation, conceived in Liberty, and dedicated\n" +
	"to the proposition that all men are created equal.\n" +
	"  Now we are engaged in a great Civil War, testing whether that\n" +
	"nation, or any nation so conceived and so dedicated, can long\n" +
	"endure.\n" +
	"  We are met on a great battle-field of that war.\n" +
	"  We have come to dedicate a portion of that field, as a final\n" +
	"resting place for those who here gave their lives that that\n" +
	"nation might live.  It is altogether fitting and proper that\n" +
	"we should do this.\n" +
	"  But, in a larger sense, we can not dedicate — we can not\n" +
	"consecrate — we can not hallow — this ground.\n" +
	"  The brave men, living and dead, who struggled here, have\n" +
	"consecrated it, far above our poor power to add or detract.\n" +
	"The world will little note, nor long remember what we say here,\n" +
	"but it can never forget what they did here.\n" +
	"  It is for us the living, rather, to be dedicated here to the\n" +
	"unfinished work which they who fought here have thus far so\n" +
	"nobly advanced.  It is rather for us to be here dedicated to\n" +
	"the great task remaining before us — that from these honored\n" +
	"dead we take increased devotion to that cause for which they\n" +
	"gave the last full measure of devotion —\n" +
	"  that we here highly resolve that these dead shall not have\n" +
	"died in vain — that this nation, under God, shall have a new\n" +
	"birth of freedom — and that government of the people, by the\n" +
	"people, for the people, shall not perish from this earth.\n" +
	"\n" +
	"Abraham Lincoln, November 19, 1863, Gettysburg, Pennsylvania\n"

var bytesSimpleGzip = []byte{ /* Hello World */
	0x1f, 0x8b, 0x08, 0x08, 0xc0, 0x6f, 0xb4, 0x63,
	0x00, 0x03, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e,
	0x74, 0x78, 0x74, 0x00, 0xf3, 0x48, 0xcd, 0xc9,
	0xc9, 0x57, 0x08, 0xcf, 0x2f, 0xca, 0x49, 0x51,
	0xe4, 0x02, 0x00, 0xdd, 0xdd, 0x14, 0x7d, 0x0d,
	0x00, 0x00, 0x00,
}

const (
	resetCount = 100
)

func runStringCompare(str string, g io.Reader, t *testing.T) {
	s := new(bytes.Buffer)
	n, err := io.Copy(s, g)

	if err == ErrUnsupportedFmt {
		t.Skip("LZ4 is not supported by current driver version, skipping this test...")
	}

	if err != nil {
		t.Fatalf("error: failed to copy data '%v'", err)
	}

	if s.String() != str {
		t.Errorf("mismatch\n***expected***\n%q:%d bytes\n\n ***received***\n%q:%d", str, len(str), s, n)
	}
}

func runStringCompressTest(str string, t *testing.T) {
	b := new(bytes.Buffer)

	z := NewWriter(b)

	z.Write([]byte(str))
	err := z.Close()

	if err != nil {
		t.Fatalf("TestInit: error failed to initialize Qat '%v'", err)
	}

	/* validate with compress/gzip */
	g, err := gzip.NewReader(b)
	if err != nil {
		t.Fatalf("TestInit: error failed to initialize compress/gzip '%v'", err)
	}

	runStringCompare(str, g, t)
}

func runStringCompressTestZSTD(str string, t *testing.T) {
	b := new(bytes.Buffer)
	z := NewWriter(b)
	z.Apply(AlgorithmOption(ZSTD))
	_, err := z.Write([]byte(str))

	if err == ErrUnsupportedFmt || err == ErrNoSwAvail {
		t.Skip("Zstd acceleration is not supported by current driver or zstd library version, skipping this test...")
	}

	if err != nil {
		t.Fatalf("TestInit: error failed to write zstd string: '%v'", err)
	}

	err = z.Close()

	if err != nil {
		t.Fatalf("TestInit: error failed to close Qat '%v'", err)
	}
	decompressedData, err := zstd.Decompress(nil, b.Bytes())
	if err != nil {
		t.Errorf("Decompression error: %v", err)
		return
	}
	if !bytes.Equal(decompressedData, []byte(str)) {
		t.Errorf("Decompressed data doesn't match the original string.\nOriginal string: %s\nDecompressed data: %s", str, string(decompressedData))
	}
}

func runStringDecompressTest(str string, t *testing.T) {
	b := new(bytes.Buffer)

	/* validate with compress/gzip */
	g := gzip.NewWriter(b)

	g.Write([]byte(str))
	err := g.Close()
	if err != nil {
		t.Fatalf("TestInit: error failed to initialize compress/gzip: '%v'", err)
	}

	z, err := NewReader(b)
	if err != nil {
		t.Fatalf("TestInit: error failed to initialize QATgo: '%v'", err)
	}

	runStringCompare(str, z, t)
}

func runStringCompressLZ4Test(str string, t *testing.T) {
	b := new(bytes.Buffer)
	// pierrec/lz4 does not handle multisession lz4 files
	// force a single session by bouncing on a single write

	z := NewWriter(b)
	z.Apply(InputBufferModeOption(Bounce), AlgorithmOption(LZ4))

	_, err := z.Write([]byte(str))
	if err == ErrUnsupportedFmt {
		t.Skip("LZ4 is not supported by current driver version, skipping this test...")
	}

	err = z.Close()

	if err != nil {
		t.Fatalf("Test: error reported by QATgo: '%v'", err)
	}

	l := lz4.NewReader(b)

	runStringCompare(str, l, t)
}

func runStringDecompressLZ4Test(str string, t *testing.T) {
	b := new(bytes.Buffer)
	l := lz4.NewWriter(b)
	l.Write([]byte(str))
	err := l.Close()
	if err != nil {
		t.Fatalf("TestInit: error failed to close LZ4 writer '%v'", err)
	}

	z, err := NewReader(b)
	if err != nil {
		t.Fatalf("TestInit: error failed to initialize QATgo: '%v'", err)
	}

	z.Apply(AlgorithmOption(LZ4))

	if err == ErrUnsupportedFmt {
		t.Skip("LZ4 is not supported by current driver version, skipping this test...")
	}
	if err != nil {
		t.Fatalf("TestInit: error failed to initialize QAT '%v'", err)
	}
	runStringCompare(str, z, t)
}

func runStringCompressRawDeflateTest(str string, t *testing.T) {
	b := new(bytes.Buffer)

	z := NewWriter(b)
	z.Apply(DeflateFmtOption(DeflateRaw))

	z.Write([]byte(str))
	err := z.Close()

	if err != nil {
		t.Fatalf("Test: error reported by QATgo: '%v'", err)
	}

	l := flate.NewReader(b)

	runStringCompare(str, l, t)
}

func runStringDecompressRawDeflateTest(str string, t *testing.T) {
	b := new(bytes.Buffer)
	f, err := flate.NewWriter(b, 1)
	if err != nil {
		t.Fatalf("TestInit: error failed to open flate writer' '%v'", err)
	}
	f.Write([]byte(str))
	err = f.Close()
	if err != nil {
		t.Fatalf("TestInit: error failed to close LZ4 writer '%v'", err)
	}

	z, err := NewReader(b)
	if err != nil {
		t.Fatalf("TestInit: error failed to initialize QATgo: '%v'", err)
	}

	z.Apply(DeflateFmtOption(DeflateRaw))

	runStringCompare(str, z, t)
}

func runBinaryDecompressFailTest(b *bytes.Buffer, t *testing.T) {
	z, err := NewReader(b)
	if err != nil {
		t.Fatalf("TestInit: error failed to initialize QATgo: '%v'", err)
	}

	out := new(bytes.Buffer)
	_, err = io.Copy(out, z)

	if err == nil {
		t.Errorf("TestFail: expected decompression failure, but received '%v'", err)
	}
}

func TestCompressShortString(t *testing.T) {
	str := string("Hello World\n")
	runStringCompressTest(str, t)
}

func TestCompressShortStringZSTD(t *testing.T) {
	str := string(strGettysBurgAddress)
	runStringCompressTestZSTD(str, t)
}

func TestCompressLongString(t *testing.T) {
	str := string(strGettysBurgAddress)
	runStringCompressTest(str, t)
}

func TestDecompressShortString(t *testing.T) {
	str := string("Hello World\n")
	runStringDecompressTest(str, t)
}

func TestDecompressLongString(t *testing.T) {
	str := string(strGettysBurgAddress)
	runStringDecompressTest(str, t)
}

func TestDecompressFailShortBinary(t *testing.T) {
	c := make([]byte, len(bytesSimpleGzip))
	copy(c, bytesSimpleGzip)
	c[0] = 0xff /* corrupt header magic 0x1f -> 0xff */
	b := bytes.NewBuffer(c)
	runBinaryDecompressFailTest(b, t)
}

func TestCompressShortStringLZ4(t *testing.T) {
	str := string("Hello World\n")
	runStringCompressLZ4Test(str, t)
}

func TestCompressLongStringLZ4(t *testing.T) {
	str := string(strGettysBurgAddress)
	runStringCompressLZ4Test(str, t)
}

func TestDecompressShortStringLZ4(t *testing.T) {
	str := string("Hello World\n")
	runStringDecompressLZ4Test(str, t)
}

func TestDecompressLongStringLZ4(t *testing.T) {
	str := string(strGettysBurgAddress)
	runStringDecompressLZ4Test(str, t)
}

func TestCompressShortStringRawDeflate(t *testing.T) {
	str := string("Hello World\n")
	runStringCompressRawDeflateTest(str, t)
}

func TestCompressLongStringRawDeflate(t *testing.T) {
	str := string(strGettysBurgAddress)
	runStringCompressRawDeflateTest(str, t)
}

func TestDecompressShortStringRawDeflate(t *testing.T) {
	str := string("Hello World\n")
	runStringDecompressRawDeflateTest(str, t)
}

func TestDecompressLongStringRawDeflate(t *testing.T) {
	str := string(strGettysBurgAddress)
	runStringDecompressRawDeflateTest(str, t)
}

func TestCloseThenWrite(t *testing.T) {
	s := "Test String..."
	b := bytes.NewBuffer([]byte(s))
	d := new(bytes.Buffer)
	z := NewWriter(d)
	z.Close()
	n, err := io.Copy(z, b)
	if n > 0 || err == nil {
		t.Error("TestFail:", n, "bytes copied on a closed Writer err:", err)
	}
}

func TestCompress0Byte(t *testing.T) {
	runStringCompressTest("", t)
}

func TestDecompress0Byte(t *testing.T) {
	runStringDecompressTest("", t)
}

func TestCompressLZ40Byte(t *testing.T) {
	runStringCompressLZ4Test("", t)
}

func TestDecompressLZ40Byte(t *testing.T) {
	runStringDecompressLZ4Test("", t)
}

func TestPanicOn0ByteDecompress(t *testing.T) {
	b := new(bytes.Buffer)
	s := new(bytes.Buffer)

	z, err := NewReader(b)
	if err != nil {
		t.Fatalf("TestInit: error failed to initialize QATgo: '%v'", err)
	}

	_, err = io.Copy(s, z)

	if err != ErrEmptyBuffer {
		t.Errorf("TestFail: expected empty buffer failure, but received '%v'", err)
	}
}

func TestReaderReset(t *testing.T) {
	b := bytes.NewBuffer([]byte(bytesSimpleGzip))
	bufLength := 128 * 1024

	z, err := NewReader(b)
	if err != nil {
		t.Fatalf("TestInit: error failed to initialize QATgo: '%v'", err)
	}

	err = z.Apply(InputBufLengthOption(bufLength), OutputBufLengthOption(bufLength))

	if err != nil {
		t.Fatalf("TestInit: error failed to apply parameters '%v'", err)
	}

	for i := 0; i < resetCount; i++ {
		d := new(bytes.Buffer)
		nr, err := io.Copy(d, z)
		if err != nil {
			t.Fatalf("TestInit: error to io.Copy err:'%v'", err)
		}

		if nr == 0 {
			t.Fatalf("TestInit: no data copied.")
		}

		err = z.Close()
		if err != nil || z.closed == false {
			t.Fatalf("TestInit: error failed to close Reader err:'%v' closed:%v", z.closed, err)
		}

		b := bytes.NewBuffer([]byte(bytesSimpleGzip))
		err = z.Reset(b)

		if err != nil || z.closed == true {
			t.Fatalf("TestInit: error failed to reset QAT err:'%v' closed:'%v'", err, z.closed)
		}
	}
}

// Test for GTO-130: Peformance Counter Reset Issue
func TestPerfCounterReset(t *testing.T) {
	s := "Test String..."

	bw := new(bytes.Buffer)
	br := new(bytes.Buffer)

	zw := NewWriter(bw)

	n, err := io.Copy(zw, bytes.NewBuffer([]byte(s)))
	if err != nil {
		t.Fatal("TestFail: copied:", n, "error with compress stream err:", err)
	}

	err = zw.Close()
	if err != nil {
		t.Fatal("TestFail: error closing Writer err:", err)
	}

	zr, _ := NewReader(bw)
	n, err = io.Copy(br, zr)

	if err != nil {
		t.Fatal("TestFail: copied:", n, "error during decompression err:", err)
	}

	bw.Reset()
	br.Reset()

	if err = zr.Reset(br); err != nil {
		t.Fatal("TestFail: error resetting Reader err:", err)
	}

	if err = zw.Reset(bw); err != nil {
		t.Fatal("TestFail: error resetting Writer err:", err)
	}

	for _, p := range [2]Perf{zr.GetPerf(), zw.GetPerf()} {
		if p.BytesIn != 0 || p.BytesOut != 0 || p.CopyTimeNS != 0 || p.EngineTimeNS != 0 || p.ReadTimeNS != 0 || p.WriteTimeNS != 0 {
			t.Fatal("TestFail: error performance counter not cleared", p)
		}
	}
}

func TestDirectCompressCRC(t *testing.T) {
	input := []byte(string(strGettysBurgAddress))
	output := make([]byte, MinBufferLength)
	crc := new(uint64)
	q, err := NewQzBinding()

	if err != nil {
		t.Fatalf("TestFail: error attempting to initialize QAT:'%v'", err)
	}

	err = q.Apply(
		AlgorithmOption(DEFLATE),
		DeflateFmtOption(DeflateGzipExt),
		CompressionLevelOption(1),
	)

	if err != nil {
		t.Fatalf("TestFail: error attempting to initialize QAT:'%v'", err)
	}

	if err = q.StartSession(); err != nil {
		t.Fatalf("TestFail: error attempting to initialize QAT:'%v'", err)
	}

	q.SetLast(true)

	in, out, err := q.CompressCRC(input, output, crc)

	if err != nil {
		t.Fatalf("TestFail: error attempting to compress with CRC err:'%v'", err)
	}
	if in <= 0 {
		t.Errorf("TestFail: no input data consumed")
	}
	if out <= 0 {
		t.Errorf("TestFail: no ouput data produced")
	}
	if *crc == 0 {
		t.Errorf("TestFail: expected changed crc value, but received 0")
	}
	q.Close()

	/* validate output with compress/gzip */
	g, err := gzip.NewReader(bytes.NewBuffer(output[:out]))
	if err != nil {
		t.Fatalf("TestInit: error failed to initialize compress/gzip '%v'", err)
	}

	runStringCompare(string(input), g, t)
}

func TestWriterApply(t *testing.T) {
	b := bytes.NewBuffer([]byte("Hello World"))
	z := NewWriter(b)

	err := z.Apply(CompressionLevelOption(5))
	if err != nil {
		t.Errorf("TestFail: initialization failure, received '%v'", err)

	}
	z.Close()
}

func TestApplyEnvOptions(t *testing.T) {
	compLvlExpected := 2
	algorithmExpected := "lz4"
	os.Setenv(envAlgorithm, algorithmExpected)
	os.Setenv(envCompressionLvl, strconv.Itoa(compLvlExpected))

	w := bytes.NewBuffer(nil)
	z := NewWriter(w)

	if z.p.Algorithm != algorithmConv[algorithmExpected] {
		t.Errorf("TestFail: algorithm %d not set by environment vars", algorithmConv[algorithmExpected])
	}

	if z.p.Level != compLvlExpected {
		t.Errorf("TestFail: expected compression level %d but got %d", compLvlExpected, z.p.Level)
	}

	err := z.Reset(w)
	if err == nil || err == ErrUnsupportedFmt {
		if z.p.Algorithm != algorithmConv[algorithmExpected] {
			t.Errorf("TestFail: algorithm %d not set by environment vars after reset", algorithmConv[algorithmExpected])
		}

		if z.p.Level != compLvlExpected {
			t.Errorf("TestFail: expected compression level %d but got %d after reset", compLvlExpected, z.p.Level)
		}
	} else {
		t.Errorf("TestFail: Writer reset failure, received %d", err)
	}
	os.Unsetenv(envAlgorithm)
	os.Unsetenv(envCompressionLvl)

	z.Close()
}

// Test for GTO-158: Close() Error Status not cleared on Reset() on Writer
func TestReset(t *testing.T) {
	bw := bytes.NewBuffer([]byte("Hello World"))
	buf := new(bytes.Buffer)
	zw := NewWriter(buf)

	zw.Reset(buf)

	if _, err := io.Copy(zw, bw); err != nil {
		t.Fatalf("TestFail: error writing after reset '%v'", err)
	}

	zw.Close()

	br := new(bytes.Buffer)
	zr, err := NewReader(buf)

	if err != nil {
		t.Fatalf("TestFail: initialization failure, received '%v'", err)
	}

	zr.Reset(buf)

	if _, err := io.Copy(br, zr); err != nil {
		t.Fatalf("TestFail: error writing after reset '%v'", err)
	}

	zr.Close()
}
