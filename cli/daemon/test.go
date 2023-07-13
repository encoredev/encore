package daemon

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/rs/zerolog/log"

	"encr.dev/cli/daemon/run"
	daemonpb "encr.dev/proto/encore/daemon"
)

// Test runs tests.
func (s *Server) Test(req *daemonpb.TestRequest, stream daemonpb.Daemon_TestServer) error {
	ctx := stream.Context()
	slog := &streamLog{stream: stream, buffered: false}
	stderr := slog.Stderr(false)
	sendErr := func(err error) {
		stderr.Write([]byte(err.Error() + "\n"))
		streamExit(stream, 1)
	}

	ctx, tracer, err := s.beginTracing(ctx, req.AppRoot, req.WorkingDir, req.TraceFile)
	if err != nil {
		sendErr(err)
		return nil
	}
	defer tracer.Close()

	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		sendErr(err)
		return nil
	}

	secrets := s.sm.Load(app)

	testCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	testResults := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				var err error
				switch recovered := recovered.(type) {
				case error:
					err = recovered
				default:
					err = fmt.Errorf("%+v", recovered)
				}
				stack := debug.Stack()
				log.Err(err).Msgf("panic during test run:\n%s", stack)
				testResults <- fmt.Errorf("panic occured within Encore during test run: %v\n%s\n", recovered, stack)
			}
		}()

		tp := run.TestParams{
			App:          app,
			WorkingDir:   req.WorkingDir,
			Environ:      req.Environ,
			Args:         req.Args,
			Secrets:      secrets,
			CodegenDebug: req.CodegenDebug,
			Stdout:       slog.Stdout(false),
			Stderr:       slog.Stderr(false),
		}
		testResults <- s.mgr.Test(testCtx, tp)
	}()

	if err := <-testResults; err != nil {
		sendErr(err)
	} else {
		streamExit(stream, 0)
	}
	return nil
}
