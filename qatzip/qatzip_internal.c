// Copyright(c) 2022-2023 Intel Corporation. All rights reserved.

#include <stdio.h>
#include <assert.h>
#include <dlfcn.h>
#include "qatzip_internal.h"
#include <time.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#ifdef ENABLE_QATGO_ZSTD
// libqatseqprod.so definitions
static qatSequenceProducer_t qatSequenceProducer = NULL;
static QZSTD_startQatDevice_t QZSTD_startQatDevice = NULL;
static QZSTD_createSeqProdState_t QZSTD_createSeqProdState = NULL;
static QZSTD_freeSeqProdState_t QZSTD_freeSeqProdState = NULL;

//libzstd.so definitions
static QZSTD_createCCtx_t QZSTD_createCCtx = NULL;
static QZSTD_createDStream_t QZSTD_createDStream = NULL;
static QZSTD_registerSequenceProducer_t QZSTD_registerSequenceProducer = NULL;
static QZSTD_CCtx_setParameter_t QZSTD_CCtx_setParameter = NULL;
static QZSTD_compressStream2_t QZSTD_compressStream2 = NULL;
static QZSTD_decompressStream_t QZSTD_decompressStream = NULL;
static QZSTD_compressBound_t QZSTD_compressBound = NULL;
static QZSTD_isError_t QZSTD_isError = NULL;
static QZSTD_freeCCtx_t QZSTD_freeCCtx = NULL;
static QZSTD_freeDStream_t QZSTD_freeDStream = NULL;
static QZSTD_getErrorName_t QZSTD_getErrorName = NULL;

static int qatzip_dload_symbols(qatzip_state_t * state, void *handle, symbol_info_t * symbols, size_t num_symbols)
{
	if (handle == NULL || symbols == NULL)
		return QZ_FAIL;

	for (size_t i = 0; i < num_symbols; i++) {
		*symbols[i].func = dlsym(handle, symbols[i].name);
		char *error = dlerror();
		if (error != NULL) {
			qatzip_debug(QDL_HIGH, state, QATHDR "failed to load symbol %s: %s\n", symbols[i].name, error);
			return QZ_NO_SW_AVAIL;
		}
	}
	return QZ_OK;
}

static int qatzip_dload_zstd_functions(qatzip_state_t * state)
{
	int status = QZ_FAIL;

	if (state == NULL)
		return QZ_FAIL;

	QzSession_ZSTD_T *session = &(state->zstd_session);

	char *zstd_lib_env = getenv("QATGO_ZSTD_LIB_PATH");
	char *qzstd_lib_env = getenv("QATGO_QZSTD_LIB_PATH");

	session->zstd_handle = dlopen(zstd_lib_env ? zstd_lib_env : ZSTD_LIB, RTLD_LAZY);
	if (!session->zstd_handle) {
		qatzip_debug(QDL_HIGH, state, QATHDR "Failed to load zstd: %s\n", dlerror());
		return QZ_FAIL;
	}
	session->qzstd_handle = dlopen(qzstd_lib_env ? qzstd_lib_env : QZSTD_LIB, RTLD_NOW);
	if (!session->qzstd_handle) {
		qatzip_debug(QDL_HIGH, state, QATHDR "failed to load qzstd: %s\n", dlerror());
		return QZ_NO_SW_AVAIL;
	}

	symbol_info_t qzstd_symbols[] = {
		{ "qatSequenceProducer", (void **)&qatSequenceProducer },
		{ "QZSTD_startQatDevice", (void **)&QZSTD_startQatDevice },
		{ "QZSTD_createSeqProdState", (void **)&QZSTD_createSeqProdState },
		{ "QZSTD_freeSeqProdState", (void **)&QZSTD_freeSeqProdState },
	};

	symbol_info_t zstd_symbols[] = {
		{ "ZSTD_createCCtx", (void **)&QZSTD_createCCtx },
		{ "ZSTD_createDStream", (void **)&QZSTD_createDStream },
		{ "ZSTD_registerSequenceProducer", (void **)&QZSTD_registerSequenceProducer },
		{ "ZSTD_CCtx_setParameter", (void **)&QZSTD_CCtx_setParameter },
		{ "ZSTD_compressStream2", (void **)&QZSTD_compressStream2 },
		{ "ZSTD_decompressStream", (void **)&QZSTD_decompressStream },
		{ "ZSTD_compressBound", (void **)&QZSTD_compressBound },
		{ "ZSTD_isError", (void **)&QZSTD_isError },
		{ "ZSTD_freeCCtx", (void **)&QZSTD_freeCCtx },
		{ "ZSTD_freeDStream", (void **)&QZSTD_freeDStream },
		{ "ZSTD_getErrorName", (void **)&QZSTD_getErrorName },
	};

	status = qatzip_dload_symbols(state, session->zstd_handle, zstd_symbols, sizeof(zstd_symbols) / sizeof(zstd_symbols[0]));
	if (status != QZ_OK) {
		return status;
	}

	status = qatzip_dload_symbols(state, session->qzstd_handle, qzstd_symbols, sizeof(qzstd_symbols) / sizeof(qzstd_symbols[0]));
	if (status != QZ_OK) {
		return status;
	}

	return QZ_OK;
}
#endif /* ENABLE_QATGO_ZSTD */

void qatzip_debug(int level, qatzip_state_t * state, char *fmt, ...)
{
	if (!state || level > state->debug) {
		return;
	}

	va_list args;
	va_start(args, fmt);
	vfprintf(stderr, fmt, args);
	va_end(args);
}

// hex dump for debug output
static void qatzip_debug_dump(int level, qatzip_state_t * state, unsigned char *buffer, unsigned int len)
{
	unsigned int pos = 0;

	if (state == NULL || buffer == NULL)
		return;

	if (state->debug < level) {
		return;
	}

	while (pos < len) {
		if (pos % 16 == 0)
			fprintf(stderr, "\n%08x  ", pos);
		fprintf(stderr, "%02x ", (unsigned int)buffer[pos++]);
		// print ascii
		if (pos % 16 == 0 || pos == len) {
			// on the last line print filler spaces
			if (pos == len) {
				unsigned int rpos = pos - 1;
				unsigned int r = (((rpos / 16) + 1) * 16) - rpos;
				for (int i = 0; i < r - 1; i++)
					fprintf(stderr, "   ");
			}
			if (pos >= 16 || pos == len) {
				fprintf(stderr, " | ");
				unsigned int base = (((pos - 1) / 16) * 16);
				unsigned int r = ((((pos - 1) / 16) + 1) * 16) - pos;
				for (int i = base; i < base + 16 - r; i++) {
					if (i >= len)
						break;
					if (buffer[i] >= 32 && buffer[i] < 127) {
						fprintf(stderr, "%c", buffer[i]);
					} else {
						fprintf(stderr, ".");
					}
				}
			}
		}
	}
	fprintf(stderr, "\n");
}

static int qatzip_zstd_init(qatzip_state_t * state)
{

#ifdef ENABLE_QATGO_ZSTD
	int ret = QZ_FAIL;

	if (state == NULL)
		return QZ_FAIL;
	QzSession_ZSTD_T *session = &(state->zstd_session);

	ret = qatzip_dload_zstd_functions(state);
	if (ret != QZ_OK) {
		return ret;
	}

	session->zstd_cctx = QZSTD_createCCtx();
	if (session->zstd_cctx == NULL) {
		qatzip_debug(QDL_HIGH, state, QATHDR "error: cannot create zstd context\n");
		return QZ_POST_PROCESS_ERROR;
	}

	if (session->level <= QAT_MAX_ZSTD_COMPRESSION_LEVEL) {
		QZSTD_startQatDevice();
		session->seqProducer = QZSTD_createSeqProdState();
		if (session->seqProducer == NULL) {
			qatzip_debug(QDL_HIGH, state, QATHDR "error: cannot create zstd seqProducer\n");
			return QZ_POST_PROCESS_ERROR;
		}
		QZSTD_registerSequenceProducer(session->zstd_cctx, session->seqProducer, qatSequenceProducer);
	} else {
		qatzip_debug(QDL_HIGH, state, QATHDR "warning: QAT acceleration disabled. Unsupported compression level %d\n", session->level);
	}

	if (QZSTD_isError(ZSTD_CCtx_setParameter(session->zstd_cctx, ZSTD_c_enableSeqProducerFallback, 1))) {
		qatzip_debug(QDL_HIGH, state, QATHDR "error: cannot enable sequence producer fallback\n");
		return QZ_POST_PROCESS_ERROR;
	}

	if (QZSTD_isError(ZSTD_CCtx_setParameter(session->zstd_cctx, ZSTD_c_compressionLevel, session->level))) {
		qatzip_debug(QDL_HIGH, state, QATHDR "error: cannot set compression level %d\n", session->level);
		return QZ_PARAMS;
	}

#else /* if ZSTD library does not support sequence producer disable at compile time */
	qatzip_debug(QDL_HIGH, state, QATHDR "error: zstd version not supported (min version is %d)\n", MIN_ZSTD_VERSION);
	return QZ_NO_SW_AVAIL;
#endif /* ENABLE_QATGO_ZSTD */
	return QZ_OK;
}

int qatzip_setup_session(qatzip_state_t * state)
{
	int status = QZ_FAIL;

	if (state == NULL)
		return QZ_FAIL;

	switch (state->algorithm) {
	case DEFLATE:
		status = qzSetupSessionDeflate(&(state->session), &(state->deflate_params));
		break;
	case LZ4:
		status = qzSetupSessionLZ4(&(state->session), &(state->lz4_params));
		break;
	case ZSTD:
		status = qatzip_zstd_init(state);
		break;
	default:
		status = QZ_UNSUPPORTED_FMT;
		break;
	}

	if (status == QZ_OK) {
		state->session_active = true;
	}
	return status;
}

qatzip_state_t *qatzip_init()
{
	int status = QZ_FAIL;
	qatzip_state_t *state = NULL;

	state = calloc(1, sizeof(qatzip_state_t));
	if (!state) {
		goto fail;
	}

	status = qzInit(&(state->session), true);
	if (status != QZ_DUPLICATE && status != QZ_OK) {
		goto fail;
	}

	status = qzGetDefaultsDeflate(&(state->deflate_params));
	if (status != QZ_OK) {
		goto fail;
	}

	status = qzGetDefaultsLZ4(&(state->lz4_params));
	if (status != QZ_OK) {
		goto fail;
	}

	state->status = QZ_OK;
	return state;

fail:
	if (state) {
		state->status = status;
	}

	return state;
}

int qatzip_compress(qatzip_state_t * state, unsigned char *in_buf, unsigned int in_size, unsigned char *out_buf, unsigned int out_size)
{
	return qatzip_compress_crc(state, in_buf, in_size, out_buf, out_size, NULL);
}

int qatzip_compress_crc(qatzip_state_t * state, unsigned char *in_buf, unsigned int in_size, unsigned char *out_buf, unsigned int out_size,
			unsigned long *crc)
{
	int status = QZ_FAIL;
	bool last = false;

	if (!state || !state->session_active) {
		qatzip_debug(QDL_HIGH, state, QATHDR "error: QAT session for state %p is not active\n", state);
		return QZ_FAIL;
	}

	QzSession_T *session = &(state->session);
	QzStream_T *stream = &(state->stream);

	stream->in = in_buf;
	stream->out = out_buf;
	stream->in_sz = in_size;
	stream->out_sz = out_size;
	last = state->last;

	/* CRC results not supported in ZSTD mode */
	if (state->algorithm == ZSTD) {
#ifdef ENABLE_QATGO_ZSTD
		size_t zstd_status = 0;
		if (out_size < QZSTD_compressBound(in_size)) {
			status = QZ_BUF_ERROR;
			goto fail;
		}

		qatzip_debug_dump(QDL_DEBUG, state, stream->in, stream->in_sz);
		qatzip_debug(QDL_HIGH, state, QATHDR "compress state: (s) i:%u o:%u last:%d\n", stream->in_sz, stream->out_sz, last);

		ZSTD_EndDirective directive;
		ZSTD_inBuffer in;
		ZSTD_outBuffer out;
		in.src = (const void *)in_buf;
		in.pos = 0;
		in.size = in_size;
		out.dst = (void *)out_buf;
		out.pos = 0;
		out.size = out_size;

		if (last) {
			directive = ZSTD_e_end;
		} else {
			directive = ZSTD_e_continue;
		}
		zstd_status = QZSTD_compressStream2(state->zstd_session.zstd_cctx, &out, &in, directive);
		if (!ZSTD_isError(zstd_status)) {
			status = QZ_OK;
		} else {
			qatzip_debug(QDL_HIGH, state, QATHDR "error: %s\n", ZSTD_getErrorName(zstd_status));
		}
		stream->in_sz = in.pos;
		stream->out_sz = out.pos;
		qatzip_debug(QDL_HIGH, state, QATHDR "compress state: (e) i:%u o:%u pi:%u po:%u ret: %d\n", stream->in_sz, stream->out_sz,
			     stream->pending_in, stream->pending_out, status);
#endif /* ENABLE_QATGO_ZSTD */
	} else {

		/* CPA_DC_FLUSH_FINAL is required for small inputs DEFLATE */
		if (state->algorithm == DEFLATE && stream->in_sz <= MIN_GZIP_SIZE) {
			qatzip_debug(QDL_HIGH, state, QATHDR "compress state: force CPA_DC_FLUSH_FINAL\n");
			last = 1;	// force CPA_DC_FLUSH_FINAL by setting last flag temporarily
		}

		qatzip_debug_dump(QDL_DEBUG, state, stream->in, stream->in_sz);
		qatzip_debug(QDL_HIGH, state, QATHDR "compress state (CRC): (s) i:%u o:%u pi:%u po:%u, last:%d, crc: %lx\n", stream->in_sz,
			     stream->out_sz, stream->pending_in, stream->pending_out, last, crc ? *crc : 0);
		status = qzCompressCrc(session, stream->in, &(stream->in_sz), stream->out, &(stream->out_sz), last, crc);
		qatzip_debug(QDL_HIGH, state, QATHDR "compress state (CRC): (e) i:%u o:%u pi:%u po:%u ret: %d, crc: %lx\n", stream->in_sz,
			     stream->out_sz, stream->pending_in, stream->pending_out, status, crc ? *crc : 0);
	}

	qatzip_debug_dump(QDL_DEBUG, state, stream->out, stream->out_sz);
fail:
	if (status != QZ_OK) {
		stream->out = NULL;
		qatzip_debug(QDL_HIGH, state, QATHDR "error: compressing input data (status: %d)\n", status);
		return status;
	}

	stream->in = NULL;
	stream->out = NULL;

	return status;
}

int qatzip_decompress(qatzip_state_t * state, unsigned char *in_buf, unsigned int in_size, unsigned char *out_buf, unsigned int out_size)
{
	int status = QZ_FAIL;
	if (!state || !state->session_active) {
		qatzip_debug(QDL_HIGH, state, QATHDR "error: QAT session for state %p is not active\n", state);
		return QZ_FAIL;
	}

	QzSession_T *session = &(state->session);
	QzStream_T *stream = &(state->stream);

	stream->in = in_buf;
	stream->out = out_buf;
	stream->in_sz = in_size;
	stream->out_sz = out_size;

	qatzip_debug(QDL_HIGH, state, QATHDR "decompress state: (s) i:%u o:%u pi:%u po:%u\n", stream->in_sz, stream->out_sz, stream->pending_in,
		     stream->pending_out);
	qatzip_debug_dump(QDL_DEBUG, state, stream->in, stream->in_sz);

	if (state->algorithm == ZSTD) {
#ifdef ENABLE_QATGO_ZSTD
		size_t zstd_status = 0;
		ZSTD_inBuffer in;
		ZSTD_outBuffer out;
		in.src = (const void *)in_buf;
		in.pos = 0;
		in.size = in_size;
		out.dst = (void *)out_buf;
		out.pos = 0;
		out.size = out_size;

		if (state->zstd_session.zstd_dctx == NULL) {
			state->zstd_session.zstd_dctx = QZSTD_createDStream();
		}

		zstd_status = QZSTD_decompressStream(state->zstd_session.zstd_dctx, &out, &in);
		if (!ZSTD_isError(zstd_status)) {
			status = QZ_OK;
		} else {
			qatzip_debug(QDL_HIGH, state, QATHDR "error: %s\n", QZSTD_getErrorName(zstd_status));
		}
		stream->in_sz = in.pos;
		stream->out_sz = out.pos;
#endif /* ENABLE_QATGO_ZSTD */
	} else {
		status = qzDecompress(session, stream->in, &(stream->in_sz), stream->out, &(stream->out_sz));
	}
	qatzip_debug(QDL_HIGH, state, QATHDR "decompress state: (e) i:%u o:%u pi:%u po:%u ret: %d\n", stream->in_sz, stream->out_sz,
		     stream->pending_in, stream->pending_out, status);
	qatzip_debug_dump(QDL_DEBUG, state, stream->out, stream->out_sz);

	if (status != QZ_OK) {
		stream->out = NULL;
		qatzip_debug(QDL_HIGH, state, QATHDR "error: decompressing input data (status: %d)\n", status);
		return status;
	}

	stream->in = NULL;
	stream->out = NULL;

	return status;
}

int qatzip_close(qatzip_state_t * state)
{
	int status = QZ_FAIL;
	if (!state) {
		return QZ_FAIL;
	}

	qatzip_debug(QDL_HIGH, state, QATHDR "closing...\n");

	if (!(state->session_active)) {
		goto done;
	}
	status = qzTeardownSession(&(state->session));
	if (status != QZ_OK) {
		goto done;
	}

	status = qzClose(&(state->session));
	if (status != QZ_OK) {
		goto done;
	}

#ifdef ENABLE_QATGO_ZSTD
	if (state->algorithm == ZSTD) {
		if (state->zstd_session.seqProducer)
			QZSTD_freeSeqProdState(state->zstd_session.seqProducer);
		if (state->zstd_session.zstd_cctx)
			QZSTD_freeCCtx(state->zstd_session.zstd_cctx);
		if (state->zstd_session.zstd_dctx)
			QZSTD_freeDStream(state->zstd_session.zstd_dctx);
		if (state->zstd_session.qzstd_handle)
			dlclose(state->zstd_session.qzstd_handle);
		if (state->zstd_session.zstd_handle)
			dlclose(state->zstd_session.zstd_handle);
	}
#endif /* ENABLE_QATGO_ZSTD */
	state->session_active = false;
	status = QZ_OK;
	qatzip_debug(QDL_HIGH, state, QATHDR "closed\n");

done:
	free(state);
	return status;
}
