package daemon

import (
	"context"
	"path/filepath"

	"encr.dev/internal/etrace"
)

func (s *Server) beginTracing(ctx context.Context, appRoot, workingDir string, traceFile *string) (context.Context, *etrace.Tracer, error) {
	if traceFile == nil {
		return ctx, nil, nil
	}

	var dst string
	if filepath.IsAbs(*traceFile) {
		dst = *traceFile
	} else {
		dst = filepath.Join(appRoot, workingDir, *traceFile)
	}
	return etrace.WithFileTracer(ctx, dst)
}
