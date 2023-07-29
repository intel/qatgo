// Copyright(c) 2022-2023 Intel Corporation. All rights reserved.

package qatzip

type Algorithm int
type Direction int
type PollingMode int
type HuffmanHdr int
type DeflateFmt int
type InputBufferMode int

const (
	DefaultCompression        = 1
	MinBufferLength           = 128 * 1024
	DefaultBufferLength       = 128 * 1024 * 1024
	DefaultBufferGrowth       = 1024 * 1024
	DefaultBounceBufferLength = 512
	MinBounceBufferLength     = 512
)

const (
	DEFLATE Algorithm = iota
	LZ4
	ZSTD
)

var algorithmConv = map[string]Algorithm{
	"deflate": DEFLATE,
	"gzip":    DEFLATE,
	"lz4":     LZ4,
	"zstd":    ZSTD,
}

func (alg Algorithm) isValid() bool {
	switch alg {
	case DEFLATE, LZ4, ZSTD:
		return true
	}
	return false
}

/* The following enums must exactly match the equivalent Enum type in QATzip.h */
const (
	/* QzDataFormat_T */
	Deflate48 DeflateFmt = iota
	DeflateGzip
	DeflateGzipExt
	DeflateRaw
)

func (fmt DeflateFmt) isValid() bool {
	switch fmt {
	case Deflate48, DeflateGzip, DeflateGzipExt, DeflateRaw:
		return true
	}
	return false
}

const (
	/* QzPollingMode_T */
	Periodical PollingMode = iota
	Busy
)

func (p PollingMode) isValid() bool {
	switch p {
	case Periodical, Busy:
		return true
	}
	return false
}

const (
	/* QzHuffmanHdr_T */
	Dynamic HuffmanHdr = iota
	Static
)

func (hdrType HuffmanHdr) isValid() bool {
	switch hdrType {
	case Dynamic, Static:
		return true
	}
	return false
}

const (
	/* QzDirection_T */
	Compress Direction = iota
	Decompress
	Both
)

func (dir Direction) isValid() bool {
	switch dir {
	case Compress, Decompress, Both:
		return true
	}
	return false
}

const (
	Reserve InputBufferMode = iota // Reserve a portion of the input buffer for last
	Bounce                         // Bounce all buffers
	Last                           // Force last=true for all buffers
	NoLast                         // Force last=false for all buffers
)

func (mode InputBufferMode) isValid() bool {
	switch mode {
	case Reserve, Bounce, Last, NoLast:
		return true
	}
	return false
}

const (
	None DebugLevel = iota
	Low
	Med
	High
	Debug
)

// Configuration parameters for QATgo compression
type params struct {
	OutputBufLength    int             // Output buffer size for QAT (for Reader and Writer)
	InputBufLength     int             // Input buffer size for QAT (for Reader)
	BufferGrowth       int             // How much to increase output buffer if required (Default 1MB)
	Direction          Direction       // Configures hardware for compress, decompress, or both (Default: Both)
	Level              int             // Compression level (Default: 1)
	Algorithm          Algorithm       // Desired compression algorithm (Default: DEFLATE)
	SwBackup           int             // Enables software fallback (Default: 1)
	MaxForks           int             // Maximum forks permitted in the current thread, 0 means no forking permitted (Default: 3)
	HwBufSize          int             // Default hardware buffer size, must be a power of 2KB (Default: 64KB)
	StreamBufSize      int             // Stream buffer size between [1KB .. 2MB - 5KB] (Default: 64KB)
	SwSwitchThreshold  int             // Threshold of compression service's input size for SW failover, if the size of input request is less (Default: 1KB)
	ReqCountThreshold  int             // Threshold for how many buffer requests it can make on a single thread (Default: 32)
	WaitCountThreshold int             // When previous try failed, wait for specific number of calls before retrying to open the device (Default: 8)
	PollingMode        PollingMode     // Settings for busy polling
	IsSensitive        int             // Enables sensitive mode (Default: 0)
	HuffmanHdr         HuffmanHdr      // Dynamic or Static Huffman headers (Default: Dynamic)
	DataFmtDeflate     DeflateFmt      // DEFLATE raw, DEFLATE with gzip or DEFLATE with gzip extended header (Default: gzip ext.)
	BounceBufferLength int             // Length of the Bounce Buffer (Default: 512)
	InputBufferMode    InputBufferMode // Settings for input buffer mode
	DebugLevel         DebugLevel      // Trace Level settings
}

func defaultParams() (p params) {
	p.Level = DefaultCompression
	p.DataFmtDeflate = DeflateGzipExt
	p.Algorithm = DEFLATE
	p.Direction = Both
	p.OutputBufLength = DefaultBufferLength
	p.InputBufLength = DefaultBufferLength
	p.BufferGrowth = DefaultBufferGrowth
	p.BounceBufferLength = DefaultBounceBufferLength
	return
}
