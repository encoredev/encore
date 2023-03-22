package daemon

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/rs/zerolog/log"

	"encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/sqldb"
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

	// Parse the app to figure out what infrastructure is needed.
	// TODO(andre) remove this
	parse, err := s.parseApp(app.Root(), req.WorkingDir, true /* parse tests */)
	if err != nil {
		sendErr(err)
		return nil
	}

	// Set up the database asynchronously since it can take a while.
	dbSetupErr := make(chan error, 1)

	var cluster *sqldb.Cluster
	if sqldb.IsUsed(parse.Meta) {
		setupCtx, setupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		cluster = s.cm.Create(setupCtx, &sqldb.CreateParams{
			ClusterID: sqldb.GetClusterID(app, sqldb.Test),
			Memfs:     true,
		})

		go func() {
			defer setupCancel()
			if _, err := cluster.Start(setupCtx); err != nil {
				dbSetupErr <- err
			} else if err := cluster.Recreate(setupCtx, req.AppRoot, nil, parse.Meta); err != nil {
				dbSetupErr <- err
			}
		}()
	}

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
			SQLDBCluster: cluster,
			WorkingDir:   req.WorkingDir,
			Environ:      req.Environ,
			Args:         req.Args,
			Parse:        parse,
			Secrets:      secrets,
			Stdout:       slog.Stdout(false),
			Stderr:       slog.Stderr(false),
		}
		testResults <- s.mgr.Test(testCtx, tp)
	}()

	select {
	case err := <-dbSetupErr:
		sendErr(err)
		return nil
	case err := <-testResults:
		if err != nil {
			sendErr(err)
		} else {
			streamExit(stream, 0)
		}
		return nil
	}
}
