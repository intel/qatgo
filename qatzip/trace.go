package qatzip

import (
	"context"
	"fmt"
	"os"
	"runtime/trace"
	"strconv"
)

type DebugLevel int

const (
	debugLevelEnv = "QATGO_DEBUG_LEVEL"
)

func getTraceLevel() (level DebugLevel) {
	e := os.Getenv(debugLevelEnv)
	l, _ := strconv.Atoi(e)
	return DebugLevel(l)
}

func traceLogf(level DebugLevel, q *QzBinding, ctx context.Context, format string, args ...any) {
	if q == nil {
		return
	}

	if q.getDebug() >= High {
		fmt.Fprintln(os.Stderr, fmt.Sprintf(format, args...))
	}

	if q.getDebug() >= level {
		trace.Log(ctx, "", fmt.Sprintf(format, args...))
	}
}

func (z *Writer) traceLogf(level DebugLevel, format string, args ...any) {
	traceLogf(level, z.q, z.ctx, format, args...)
}

func (z *Reader) traceLogf(level DebugLevel, format string, args ...any) {
	traceLogf(level, z.q, z.ctx, format, args...)
}
