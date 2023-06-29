// Copyright(c) 2022-2023 Intel Corporation. All rights reserved.

package qatzip

import "errors"

var (
	QatErrHdr                  = string("QATzip error: ")
	ErrParams                  = errors.New(QatErrHdr + "invalid parameter in function call")
	ErrFail                    = errors.New(QatErrHdr + "unspecified error")
	ErrBuffer                  = errors.New(QatErrHdr + "insufficient buffer error")
	ErrData                    = errors.New(QatErrHdr + "input data was corrupted")
	ErrTimeout                 = errors.New(QatErrHdr + "operation timed out")
	ErrIntegrity               = errors.New(QatErrHdr + "integrity checked failed")
	ErrNoHw                    = errors.New(QatErrHdr + "using SW: No QAT HW detected")
	ErrNoMDrv                  = errors.New(QatErrHdr + "using SW: No memory driver detected")
	ErrNoInstAttached          = errors.New(QatErrHdr + "using SW: Could not attach to an instance")
	ErrLowMem                  = errors.New(QatErrHdr + "using SW: Not enough pinned memory")
	ErrLowDestMem              = errors.New(QatErrHdr + "using SW: Not enough pinned memory for dest buffer")
	ErrUnsupportedFmt          = errors.New(QatErrHdr + "using SW: QAT device does not support data format")
	ErrNone                    = errors.New(QatErrHdr + "device uninitialized")
	ErrNoSwHw                  = errors.New(QatErrHdr + "not using SW: No QAT HW detected")
	ErrNoSwMDrv                = errors.New(QatErrHdr + "not using SW: No memory driver detected")
	ErrNoSwNoInst              = errors.New(QatErrHdr + "not using SW: Could not attach to instance")
	ErrNoSwLowMem              = errors.New(QatErrHdr + "not using SW: not enough pinned memory")
	ErrNoSwAvail               = errors.New(QatErrHdr + "session may require software, but no software is available")
	ErrNoSwUnsupportedFmt      = errors.New(QatErrHdr + "not using SW: QAT device does not support data format")
	ErrPostProcess             = errors.New(QatErrHdr + "using post process: post process callback encountered an error")
	ErrMetaDataOverflow        = errors.New(QatErrHdr + "insufficent memory allocated for metadata")
	ErrOutOfRange              = errors.New(QatErrHdr + "metadata block_num specified is out of range")
	ErrNotSupported            = errors.New(QatErrHdr + "request not supported")
	ErrParamOutputBufLength    = errors.New(QatErrHdr + "invalid size for output buffer length")
	ErrParamInputBufLength     = errors.New(QatErrHdr + "invalid size for input buffer length")
	ErrParamBufferGrowth       = errors.New(QatErrHdr + "invalid size for buffer Growth")
	ErrParamBounceBufferLength = errors.New(QatErrHdr + "invalid size for bounce buffer length")
	ErrParamAlgorithm          = errors.New(QatErrHdr + "invalid algorithm type")
	ErrParamDirection          = errors.New(QatErrHdr + "invalid direction")
	ErrParamDataFmtDeflate     = errors.New(QatErrHdr + "invalid deflate format type")
	ErrParamHuffmanHdr         = errors.New(QatErrHdr + "invalid huffman header type")
	ErrParamPollingMode        = errors.New(QatErrHdr + "invalid polling mode")
	ErrParamCompressionLevel   = errors.New(QatErrHdr + "invalid compression level")
	ErrWriterClosed            = errors.New(QatErrHdr + "cannot write to closed writer")
	ErrEmptyBuffer             = errors.New(QatErrHdr + "empty buffer")
	ErrNoMem                   = errors.New(QatErrHdr + "out of memory")
	ErrInputBufferMode         = errors.New(QatErrHdr + "invalid input buffer mode")
	ErrApplyPostInit           = errors.New(QatErrHdr + "cannot apply options after Reset() or I/O")
	ErrApplyInvalidType        = errors.New(QatErrHdr + "option appied to incorrect type")
)

func Error(errorCode int) (err error) {
	switch errorCode {
	case 0: /* QZ_OK */
		err = nil
	case 1: /* QZ_DUPLICATE */
		err = nil
	case 2: /* QZ_FORCE_SW */
		err = nil
	case -1:
		err = ErrParams
	case -2:
		err = ErrFail
	case -3:
		err = ErrBuffer
	case -4:
		err = ErrData
	case -5:
		err = ErrTimeout
	case -100:
		err = ErrIntegrity
	case 11:
		err = ErrNoHw
	case 12:
		err = ErrNoMDrv
	case 13:
		err = ErrNoInstAttached
	case 14:
		err = ErrLowMem
	case 15:
		err = ErrLowDestMem
	case 16:
		err = ErrUnsupportedFmt
	case 100:
		err = ErrNone
	case -101:
		err = ErrNoSwHw
	case -102:
		err = ErrNoSwMDrv
	case -103:
		err = ErrNoSwNoInst
	case -104:
		err = ErrNoSwLowMem
	case -105:
		err = ErrNoSwAvail
	case -116:
		err = ErrNoSwUnsupportedFmt
	case -117:
		err = ErrPostProcess
	case -118:
		err = ErrMetaDataOverflow
	case -119:
		err = ErrOutOfRange
	case -200:
		err = ErrNotSupported
	default:
		err = ErrFail
	}

	return err
}
