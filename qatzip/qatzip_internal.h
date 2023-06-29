// Copyright(c) 2022-2023 Intel Corporation. All rights reserved.

#ifndef __QATZIP_INTERNAL_H__
#define __QATZIP_INTERNAL_H__
#define ZSTD_STATIC_LINKING_ONLY

#define QAT_MAX_ZSTD_COMPRESSION_LEVEL 12
#define MIN_ZSTD_VERSION 10505
#define ZSTD_LIB "libzstd.so"
#define QZSTD_LIB "libqatseqprod.so"

#include "qatzip.h"
#include <stdarg.h>
#include <stdbool.h>

enum Algorithm { DEFLATE, LZ4, ZSTD };

typedef struct {
	const char *name;
	void **func;
} symbol_info_t;

/* Check requirements for zstd */
#if __has_include(<zstd.h>)
#include <zstd.h>

/* QAT requires at least ZSTD 1.5.5 to support acceleration */
#if ZSTD_VERSION_NUMBER >= MIN_ZSTD_VERSION
#define ENABLE_QATGO_ZSTD
#endif

// libqatseqprod.so definitions
typedef size_t (*qatSequenceProducer_t)(void *, ZSTD_Sequence *, size_t, const void *, size_t, const void *, size_t, int, size_t);
typedef void (*QZSTD_startQatDevice_t)();
typedef void *(*QZSTD_createSeqProdState_t)();
typedef void (*QZSTD_freeSeqProdState_t)(void *);

//libzstd.so definitions
typedef ZSTD_CCtx *(*QZSTD_createCCtx_t)();
typedef ZSTD_DStream *(*QZSTD_createDStream_t)();
typedef void (*QZSTD_registerSequenceProducer_t)(ZSTD_CCtx *, void *, void *);
typedef int (*QZSTD_CCtx_setParameter_t)(ZSTD_CCtx *, int, int);
typedef size_t (*QZSTD_compressStream2_t)(ZSTD_CCtx *, ZSTD_outBuffer *, ZSTD_inBuffer *, ZSTD_EndDirective);
typedef size_t (*QZSTD_decompressStream_t)(ZSTD_DStream *, ZSTD_outBuffer *, ZSTD_inBuffer *);
typedef size_t (*QZSTD_compressBound_t)(size_t);
typedef size_t (*QZSTD_isError_t)(size_t);
typedef size_t (*QZSTD_freeCCtx_t)(ZSTD_CCtx *);
typedef size_t (*QZSTD_freeDStream_t)(ZSTD_DStream *);
typedef const char *(*QZSTD_getErrorName_t)(size_t);

#endif /* ZSTD_VERSION_NUMBER >= MIN_ZSTD_VERSION */

typedef struct {
#ifdef ENABLE_QATGO_ZSTD
	ZSTD_CCtx *zstd_cctx;
	ZSTD_DStream *zstd_dctx;
#endif				/* ENABLE_QATGO_ZSTD */
	void *seqProducer;
	void *zstd_handle;
	void *qzstd_handle;
	int level;
} QzSession_ZSTD_T;

typedef struct {
	QzSession_T session;
	QzSessionParamsDeflate_T deflate_params;
	QzSessionParamsLZ4_T lz4_params;
	QzSession_ZSTD_T zstd_session;
	QzStream_T stream;
	int algorithm;
	int last;
	bool session_active;
	int debug;
	int status;

} qatzip_state_t;

#define MIN_GZIP_SIZE 1024

qatzip_state_t *qatzip_init();
int qatzip_setup_session(qatzip_state_t * state);
int qatzip_compress(qatzip_state_t * state, unsigned char *in_buf, unsigned int in_size, unsigned char *out_buf, unsigned int out_size);
int qatzip_compress_crc(qatzip_state_t * state, unsigned char *in_buf,
			unsigned int in_size, unsigned char *out_buf, unsigned int out_size, unsigned long *crc);
int qatzip_decompress(qatzip_state_t * state, unsigned char *in_buf, unsigned int in_size, unsigned char *out_buf, unsigned int out_size);
int qatzip_close(qatzip_state_t * state);
void qatzip_debug(int level, qatzip_state_t * state, char *fmt, ...);

#define QATHDR "QATzip (internal): "
/* Debug Levels */
#define QDL_NONE 0
#define QDL_LOW 1
#define QDL_MED 2
#define QDL_HIGH 3
#define QDL_DEBUG 4

#endif /* __QATZIP_INTERNAL_H__ */
