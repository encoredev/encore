package parsectx

import (
	"context"
	"go/token"
	"time"

	"github.com/rs/xid"
	"github.com/rs/zerolog"

	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
)

// Context holds all the context for parsing.
type Context struct {
	// Ctx provides cancellation.
	Ctx context.Context

	// Log is the configured logger.
	Log zerolog.Logger

	// Build controls what files to build.
	Build BuildInfo

	// MainModuleDir is the directory containing the main module.
	MainModuleDir paths.FS

	// FS holds the fileset used for parsing.
	FS *token.FileSet

	// ParseTests controls whether to parse test files.
	ParseTests bool

	// Errs contains encountered errors.
	Errs *perr.List
}

// BuildInfo represents the information needed to parse and build an Encore application.
type BuildInfo struct {
	GOARCH string // target architecture
	GOOS   string // target operating system
	GOROOT string // GOROOT to use

	BuildTags  []string // additional build tags to set
	CgoEnabled bool
}

// Trace traces the execution of a function.
// It emits trace-level log messages, using the given message and key-value pairs.
// It returns the logger for logging additional information during the processing.
//
// Usage:
//
//	tr := ctx.Trace("operation-name", "key", value)
//	// ... invoke tr.Emit(...) to log additional information
//	defer tr.Done()
func (c *Context) Trace(op string, kvs ...any) *TraceLogger {
	// If we're not tracing, do nothing.
	if c.Log.GetLevel() > zerolog.TraceLevel {
		return nil
	}

	log := c.Log.With().Str("op", op).Str("op_id", "op_"+xid.New().String()).Logger()
	log.Trace().Caller(1).Fields(kvs).Msg("start")
	now := time.Now()
	return &TraceLogger{log: log, start: now, prev: now}
}

type TraceLogger struct {
	log   zerolog.Logger
	start time.Time
	prev  time.Time
}

func (t *TraceLogger) Done(kvs ...any) {
	if t == nil {
		return
	}
	t.Emit("done", kvs...)
}

func (t *TraceLogger) Emit(msg string, kvs ...any) {
	if t == nil {
		return
	}
	now := time.Now()
	t.prev = now
	t.log.Trace().
		Caller(1).
		Dur("from_start", now.Sub(t.start)).
		Dur("from_prev", now.Sub(t.prev)).
		Fields(kvs).
		Msg(msg)
}
