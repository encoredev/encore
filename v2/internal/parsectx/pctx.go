package parsectx

import (
	"context"
	"fmt"
	"go/token"
	"runtime/trace"
	"strings"
	"time"

	"github.com/rs/xid"
	"github.com/rs/zerolog"

	"encr.dev/internal/paths"
	"encr.dev/pkg/option"
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
	GOARCH string   // target architecture
	GOOS   string   // target operating system
	GOROOT paths.FS // GOROOT to use

	EncoreRuntime paths.FS // Encore runtime to use

	BuildTags  []string // additional build tags to set
	CgoEnabled bool

	// StaticLink enables static linking of C libraries.
	StaticLink bool

	// Debug enables compiling in debug mode.
	Debug bool

	// Revision specifies the revision of the build.
	Revision string

	// UncommittedChanges, if true, specifies there are uncommitted changes
	// part of the build .
	UncommittedChanges bool

	// MainPkg is the existing main package to use, if any.
	// If None a main package is generated.
	MainPkg option.Option[paths.Pkg]
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
	if c.Log.GetLevel() > zerolog.TraceLevel && !trace.IsEnabled() {
		return nil
	}

	taskCtx, task := trace.NewTask(c.Ctx, op)

	log := c.Log.With().Str("op", op).Str("op_id", "op_"+xid.New().String()).Logger()
	log.Trace().Caller(1).Fields(kvs).Msg("start")
	now := time.Now()
	return &TraceLogger{taskCtx: taskCtx, task: task, log: log, start: now, prev: now}
}

type TraceLogger struct {
	taskCtx context.Context
	task    *trace.Task
	log     zerolog.Logger
	start   time.Time
	prev    time.Time
}

func (t *TraceLogger) Done(kvs ...any) {
	if t == nil {
		return
	}
	t.emit("done", kvs)
	t.task.End()
}

func (t *TraceLogger) Emit(msg string, kvs ...any) {
	if t == nil {
		return
	}
	t.emit(msg, kvs)

	// Write to the trace log if tracing is enabled.
	if trace.IsEnabled() {
		var logMsg strings.Builder
		logMsg.WriteString(msg)
		for i := 0; i < len(kvs)/2; i++ {
			key := kvs[2*i]
			value := kvs[2*i+1]
			fmt.Fprintf(&logMsg, " %v=%v", key, value)
		}

		trace.Log(t.taskCtx, "", logMsg.String())
	}
}

func (t *TraceLogger) emit(msg string, kvs []any) {
	now := time.Now()
	t.prev = now
	t.log.Trace().
		Caller(1).
		Dur("from_start", now.Sub(t.start)).
		Dur("from_prev", now.Sub(t.prev)).
		Fields(kvs).
		Msg(msg)
}
