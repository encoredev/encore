package parsectx

import (
	"context"
	"go/token"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/rs/zerolog"

	"encr.dev/parser2/internal/perr"
)

// Context holds all the context for parsing.
type Context struct {
	// Ctx provides cancellation.
	Ctx context.Context

	// Log is the configured logger.
	Log zerolog.Logger

	// Build controls what files to build.
	Build BuildInfo

	// FS holds the fileset used for parsing.
	FS *token.FileSet

	// ParseTests reports whether to parse test files.
	ParseTests bool

	// Errs contains encountered errors.
	Errs *perr.List

	// c is the test runner. It is nil when not running tests.
	c *qt.C
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
// Usage:
//
//	defer ctx.Trace("doing something", "key", value)()
func (c *Context) Trace(msg string, kvs ...any) func() {
	// If we're not tracing, do nothing.
	if c.Log.GetLevel() > zerolog.TraceLevel {
		return func() {}
	}

	// If we're running tests, mark this function as a testing helper.
	if c.c != nil {
		c.c.Helper()
	}

	start := time.Now()
	c.Log.Trace().Caller(1).Time("start", start).Fields(kvs).Msg("start: " + msg)
	return func() {
		end := time.Now()
		c.Log.Trace().Caller(1).Time("end", end).Dur("duration", end.Sub(start)).Fields(kvs).Msg("end:   " + msg)
	}
}
