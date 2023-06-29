package qatzip

type Option func(a applier) error

type applier interface {
	Apply(...Option) error
}

func booltoInt(b bool) (v int) {
	if b {
		v = 1
	}
	return
}

// Compression level setting (Writer)
func CompressionLevelOption(level int) Option {
	return func(a applier) error {
		if level <= 0 {
			return ErrParamCompressionLevel
		}

		switch z := a.(type) {
		case *Writer:
			z.p.Level = level
		case *QzBinding:
			z.p.Level = level
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Algorithm setting [DEFLATE, lz4, zstd]
func AlgorithmOption(alg Algorithm) Option {
	return func(a applier) error {
		if !alg.isValid() {
			return ErrParamAlgorithm
		}

		switch z := a.(type) {
		case *Reader:
			z.p.Algorithm = alg
		case *Writer:
			z.p.Algorithm = alg
		case *QzBinding:
			z.p.Algorithm = alg
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Polling mode setting [periodical, busy]
func PollingModeOption(mode PollingMode) Option {
	return func(a applier) error {
		if !mode.isValid() {
			return ErrParamPollingMode
		}

		switch z := a.(type) {
		case *Reader:
			z.p.PollingMode = mode
		case *Writer:
			z.p.PollingMode = mode
		case *QzBinding:
			z.p.PollingMode = mode
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// DEFLATE format setting [raw, gzip, gzip extended]
func DeflateFmtOption(fmt DeflateFmt) Option {
	return func(a applier) error {
		if !fmt.isValid() {
			return ErrParamDataFmtDeflate
		}

		switch z := a.(type) {
		case *Reader:
			z.p.DataFmtDeflate = fmt
		case *Writer:
			z.p.DataFmtDeflate = fmt
		case *QzBinding:
			z.p.DataFmtDeflate = fmt
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Dynamic or static Huffman headers setting
func HuffmanHdrOption(hdrType HuffmanHdr) Option {
	return func(a applier) error {
		if !hdrType.isValid() {
			return ErrParamHuffmanHdr
		}

		switch z := a.(type) {
		case *Writer:
			z.p.HuffmanHdr = hdrType
		case *QzBinding:
			z.p.HuffmanHdr = hdrType
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// I/O direction setting [compress, decompress, both]
func DirOption(dir Direction) Option {
	return func(a applier) error {
		if !dir.isValid() {
			return ErrParamDirection
		}

		switch z := a.(type) {
		case *Reader:
			if dir == Compress {
				return ErrParams
			}
			z.p.Direction = dir
		case *Writer:
			if dir == Decompress {
				return ErrParams
			}
			z.p.Direction = dir
		case *QzBinding:
			z.p.Direction = dir
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Input buffer mode setting (Writer) [Reserve, Bounce, Last, NoLast]
func InputBufferModeOption(mode InputBufferMode) Option {
	return func(a applier) error {
		if !mode.isValid() {
			return ErrInputBufferMode
		}

		switch z := a.(type) {
		case *Writer:
			z.p.InputBufferMode = mode
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Debug level option [None, Low, Med, High, Debug]
func DebugLevelOption(level DebugLevel) Option {
	return func(a applier) error {
		switch z := a.(type) {
		case *Reader:
			z.p.DebugLevel = level
		case *Writer:
			z.p.DebugLevel = level
		case *QzBinding:
			z.p.DebugLevel = level
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Output buffer size (Reader, Writer)
func OutputBufLengthOption(len int) Option {
	return func(a applier) error {
		if len < MinBufferLength {
			return ErrParamOutputBufLength
		}

		switch z := a.(type) {
		case *Writer:
			z.p.OutputBufLength = len
		case *Reader:
			z.p.OutputBufLength = len
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Input buffer length (Reader)
func InputBufLengthOption(len int) Option {
	return func(a applier) error {
		if len < MinBufferLength {
			return ErrParamInputBufLength
		}

		switch z := a.(type) {
		case *Reader:
			z.p.InputBufLength = len
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Length of the bounce buffer (Writer)
func BounceBufferLengthOption(len int) Option {
	return func(a applier) error {
		if len < MinBounceBufferLength {
			return ErrParamBounceBufferLength
		}
		switch z := a.(type) {
		case *Writer:
			z.p.BounceBufferLength = len
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// If output buffer is too small (see QZ_BUF_ERROR) increase size of output buffer a factor of len and retry
// (Reader/Writer)
func BufferGrowthOption(len int) Option {
	return func(a applier) error {
		if len < MinBufferLength {
			return ErrParamBufferGrowth
		}

		switch z := a.(type) {
		case *Reader:
			z.p.BufferGrowth = len
		case *Writer:
			z.p.BufferGrowth = len
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Software fallback option
func SwBackupOption(enable bool) Option {
	return func(a applier) error {
		v := booltoInt(enable)
		switch z := a.(type) {
		case *Reader:
			z.p.SwBackup = v
		case *Writer:
			z.p.SwBackup = v
		case *QzBinding:
			z.p.SwBackup = v
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Sensitve mode option
func SensitiveOption(enable bool) Option {
	return func(a applier) error {
		v := booltoInt(enable)
		switch z := a.(type) {
		case *Reader:
			z.p.IsSensitive = v
		case *Writer:
			z.p.IsSensitive = v
		case *QzBinding:
			z.p.IsSensitive = v
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Maximum forks permitted in the current thread
func MaxForksOption(max int) Option {
	return func(a applier) error {
		switch z := a.(type) {
		case *Reader:
			z.p.MaxForks = max
		case *Writer:
			z.p.MaxForks = max
		case *QzBinding:
			z.p.MaxForks = max
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Hardware buffer size [must be a power of 2KB]
func HwBufSizeOption(size int) Option {
	return func(a applier) error {
		switch z := a.(type) {
		case *Reader:
			z.p.HwBufSize = size
		case *Writer:
			z.p.HwBufSize = size
		case *QzBinding:
			z.p.HwBufSize = size
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Stream buffer size between [1KB .. 2MB - 5KB]
func StreamBufSizeOption(size int) Option {
	return func(a applier) error {
		switch z := a.(type) {
		case *Reader:
			z.p.StreamBufSize = size
		case *Writer:
			z.p.StreamBufSize = size
		case *QzBinding:
			z.p.StreamBufSize = size
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// If input size is less than threshold size switch to SW
func SwSwitchThresholdOption(size int) Option {
	return func(a applier) error {
		switch z := a.(type) {
		case *Reader:
			z.p.SwSwitchThreshold = size
		case *Writer:
			z.p.SwSwitchThreshold = size
		case *QzBinding:
			z.p.SwSwitchThreshold = size
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Threshold for how many buffer requests QATzip can make on a single thread
func ReqCountThresholdOption(n int) Option {
	return func(a applier) error {
		switch z := a.(type) {
		case *Reader:
			z.p.ReqCountThreshold = n
		case *Writer:
			z.p.ReqCountThreshold = n
		case *QzBinding:
			z.p.ReqCountThreshold = n
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

// Retry count for QATzip initialization if failure occurs
func WaitCountThresholdOption(n int) Option {
	return func(a applier) error {
		switch z := a.(type) {
		case *Reader:
			z.p.WaitCountThreshold = n
		case *Writer:
			z.p.WaitCountThreshold = n
		case *QzBinding:
			z.p.WaitCountThreshold = n
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}
