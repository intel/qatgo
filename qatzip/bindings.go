// Copyright(c) 2022-2023 Intel Corporation. All rights reserved.

package qatzip

/*
#include "qatzip_internal.h"
#cgo pkg-config: qatzip
#cgo LDFLAGS: -ldl
*/
import "C"

const (
	DEFLATE_ID uint8 = C.QZ_DEFLATE
	LZ4_ID     uint8 = C.QZ_LZ4
)

// Manages QATzip session state
type QzBinding struct {
	state  *C.qatzip_state_t
	p      params
	closed bool
}

func (q *QzBinding) getDebug() DebugLevel {
	return DebugLevel(q.state.debug)
}

// Create QATzip session state
func NewQzBinding() (q *QzBinding, err error) {
	q = new(QzBinding)
	q.state = C.qatzip_init()

	if q.state == nil {
		return nil, ErrNoMem
	}

	if err = Error(int(q.state.status)); err != nil {
		q.Close()
		return nil, err
	}

	return q, err
}

func (q *QzBinding) setParams(p params) {
	q.p = p
}

// Start QATzip session
func (q *QzBinding) StartSession() (err error) {
	var commonParams *C.QzSessionParamsCommon_T

	q.state.debug = C.int(q.p.DebugLevel)

	switch q.p.Algorithm {
	case DEFLATE:
		commonParams = &q.state.deflate_params.common_params
		commonParams.comp_algorithm = C.uchar(DEFLATE_ID)
		q.state.algorithm = C.int(DEFLATE)
	case LZ4:
		commonParams = &q.state.lz4_params.common_params
		commonParams.comp_algorithm = C.uchar(LZ4_ID)
		q.state.algorithm = C.int(LZ4)
	case ZSTD:
		q.state.zstd_session.level = C.int(q.p.Level)
		q.state.algorithm = C.int(ZSTD)
	default:
		return ErrParams
	}

	// initialize common QAT parameters
	if commonParams != nil {
		if q.p.Direction != 0 {
			commonParams.direction = C.QzDirection_T(q.p.Direction)
		}
		if q.p.Level != 0 {
			commonParams.comp_lvl = C.uint(q.p.Level)
		}
		if q.p.SwBackup != 0 {
			commonParams.sw_backup = C.uchar(q.p.SwBackup)
		}
		if q.p.MaxForks != 0 {
			commonParams.max_forks = C.uint(q.p.MaxForks)
		}
		if q.p.HwBufSize != 0 {
			commonParams.hw_buff_sz = C.uint(q.p.HwBufSize)
		}
		if q.p.StreamBufSize != 0 {
			commonParams.strm_buff_sz = C.uint(q.p.StreamBufSize)
		}
		if q.p.SwSwitchThreshold != 0 {
			commonParams.input_sz_thrshold = C.uint(q.p.SwSwitchThreshold)
		}
		if q.p.ReqCountThreshold != 0 {
			commonParams.req_cnt_thrshold = C.uint(q.p.ReqCountThreshold)
		}
		if q.p.WaitCountThreshold != 0 {
			commonParams.wait_cnt_thrshold = C.uint(q.p.WaitCountThreshold)
		}
		if q.p.IsSensitive != 0 {
			commonParams.is_sensitive_mode = C.uint(q.p.IsSensitive)
		}
		if q.p.PollingMode != Periodical {
			commonParams.polling_mode = C.QzPollingMode_T(q.p.PollingMode)
		}
		if q.p.HuffmanHdr != Dynamic {
			q.state.deflate_params.huffman_hdr = C.QzHuffmanHdr_T(q.p.HuffmanHdr)
		}
		if q.p.DataFmtDeflate != DeflateGzipExt {
			q.state.deflate_params.data_fmt = C.QzDataFormat_T(q.p.DataFmtDeflate)
		}
	}

	status := C.qatzip_setup_session(q.state)
	if status != 0 {
		return Error(int(status))
	}

	return nil
}

// End QATzip session
func (q *QzBinding) Close() (err error) {
	if q.closed {
		return nil
	}

	q.closed = true
	status := int(C.qatzip_close(q.state))
	if status != 0 {
		return Error(status)
	}
	return nil
}

// QATzip compress (in = input buffer, out = output buffer, c = consumed, p = produced)
func (q *QzBinding) Compress(in []byte, out []byte) (c int, p int, err error) {
	if len(in) == 0 {
		err = ErrEmptyBuffer
		return
	}

	status := int(C.qatzip_compress(q.state,
		(*C.uchar)(&in[0]), C.uint(len(in)),
		(*C.uchar)(&out[0]), C.uint(len(out))))

	c = int(q.state.stream.in_sz)
	p = int(q.state.stream.out_sz)
	err = Error(status)

	return
}

// QATzip compress with CRC (in = input buffer, out = output buffer, crc = crc value, c = consumed, p = produced)
func (q *QzBinding) CompressCRC(in []byte, out []byte, crc *uint64) (c int, p int, err error) {
	if len(in) == 0 {
		err = ErrEmptyBuffer
		return
	}

	status := int(C.qatzip_compress_crc(q.state,
		(*C.uchar)(&in[0]), C.uint(len(in)),
		(*C.uchar)(&out[0]), C.uint(len(out)), (*C.ulong)(crc)))

	c = int(q.state.stream.in_sz)
	p = int(q.state.stream.out_sz)
	err = Error(status)

	return
}

// Set last flag for QATzip compress
func (q *QzBinding) SetLast(enable bool) {
	if enable {
		q.state.last = C.int(1)
	} else {
		q.state.last = C.int(0)
	}
}

// QATzip decompress (in = input buffer, out = output buffer, c = consumed, p = produced)
func (q *QzBinding) Decompress(in []byte, out []byte) (c int, p int, err error) {
	if len(in) == 0 {
		err = ErrEmptyBuffer
		return
	}
	status := int(C.qatzip_decompress(q.state,
		(*C.uchar)(&in[0]), C.uint(len(in)),
		(*C.uchar)(&out[0]), C.uint(len(out))))

	c = int(q.state.stream.in_sz)
	p = int(q.state.stream.out_sz)
	err = Error(status)

	return
}

// Apply options to QATzip session state
func (q *QzBinding) Apply(options ...Option) (err error) {
	for _, op := range options {
		if err = op(q); err != nil {
			return
		}
	}
	return
}
