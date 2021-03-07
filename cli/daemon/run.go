package daemon

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"encr.dev/cli/daemon/internal/appfile"
	"encr.dev/cli/daemon/internal/manifest"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/parser"
	daemonpb "encr.dev/proto/encore/daemon"
	meta "encr.dev/proto/encore/parser/meta/v1"
	"github.com/rs/zerolog/log"
	"golang.org/x/mod/modfile"
)

// Run runs the application.
func (s *Server) Run(req *daemonpb.RunRequest, stream daemonpb.Daemon_RunServer) error {
	sendErr := func(err error) {
		stream.Send(&daemonpb.RunMessage{
			Msg: &daemonpb.RunMessage_Output{Output: &daemonpb.CommandOutput{
				Stderr: []byte(err.Error() + "\n"),
			}},
		})
		stream.Send(&daemonpb.RunMessage{
			Msg: &daemonpb.RunMessage_Exit{Exit: &daemonpb.CommandExit{
				Code: 1,
			}},
		})
	}

	// Prefetch secrets if the app is linked.
	if appSlug, err := appfile.Slug(req.AppRoot); err == nil && appSlug != "" {
		s.sm.Prefetch(appSlug)
	}

	// Parse the app to figure out what infrastructure is needed.
	parse, err := s.parseApp(req.AppRoot, req.WorkingDir, false)
	if err != nil {
		sendErr(err)
		return nil
	}

	man, err := manifest.ReadOrCreate(req.AppRoot)
	if err != nil {
		sendErr(err)
		return nil
	}
	s.cacheAppRoot(man.AppID, req.AppRoot)

	clusterID := man.AppID
	dbSetupErr := make(chan error, 1)

	// Set up the database only if the app requires it.
	if requiresSQLDB(parse.Meta) {
		cluster := s.cm.Init(stream.Context(), &sqldb.InitParams{
			ClusterID: clusterID,
			Meta:      parse.Meta,
			Memfs:     false,
		})
		// Set up the database asynchronously since it can take a while.
		go func() {
			if err := cluster.Start(); err != nil {
				dbSetupErr <- err
			} else if err := cluster.CreateAndMigrate(stream.Context(), req.AppRoot, parse.Meta); err != nil {
				dbSetupErr <- err
			}
		}()
	}

	// Hold the stream mutex so we can set up the stream map
	// before output starts.
	s.mu.Lock()
	run, err := s.mgr.Start(stream.Context(), run.StartParams{
		AppRoot:     req.AppRoot,
		AppID:       man.AppID,
		WorkingDir:  req.WorkingDir,
		DBClusterID: clusterID,
		Parse:       parse,
		Watch:       req.Watch,
	})
	if err != nil {
		s.mu.Unlock()
		sendErr(err)
		return nil
	}
	s.streams[run.ID] = stream
	s.mu.Unlock()

	pid := run.Proc().Pid
	_ = stream.Send(&daemonpb.RunMessage{
		Msg: &daemonpb.RunMessage_Started{Started: &daemonpb.RunStarted{
			RunId: run.ID,
			Pid:   int32(pid),
			Port:  int32(run.Port),
		}},
	})

	// Wait for the run to close, or the database setup to fail.
	select {
	case <-run.Done():
	case err := <-dbSetupErr:
		log.Error().Err(err).Str("appID", run.AppID).Msg("failed to setup db")
		sendErr(fmt.Errorf("Database setup failed: %v", err))
	}

	s.mu.Lock()
	delete(s.streams, run.ID)
	s.mu.Unlock()
	return nil
}

// Test runs tests.
func (s *Server) Test(req *daemonpb.TestRequest, stream daemonpb.Daemon_TestServer) error {
	sendErr := func(err error) {
		stream.Send(&daemonpb.CommandMessage{
			Msg: &daemonpb.CommandMessage_Output{Output: &daemonpb.CommandOutput{
				Stderr: []byte(err.Error() + "\n"),
			}},
		})
		streamExit(stream, 1)
	}

	// Prefetch secrets if the app is linked.
	if appSlug, err := appfile.Slug(req.AppRoot); err == nil && appSlug != "" {
		s.sm.Prefetch(appSlug)
	}

	// Parse the app to figure out what infrastructure is needed.
	parse, err := s.parseApp(req.AppRoot, req.WorkingDir, true /* parse tests */)
	if err != nil {
		sendErr(err)
		return nil
	}

	man, err := manifest.ReadOrCreate(req.AppRoot)
	if err != nil {
		sendErr(err)
		return nil
	}
	s.cacheAppRoot(man.AppID, req.AppRoot)

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	clusterID := man.AppID + "-test"
	cluster := s.cm.Init(stream.Context(), &sqldb.InitParams{
		ClusterID: clusterID,
		Memfs:     true,
		Meta:      parse.Meta,
	})

	// Set up the database asynchronously since it can take a while.
	dbSetupErr := make(chan error, 1)
	go func() {
		err := cluster.Recreate(stream.Context(), req.AppRoot, nil, parse.Meta)
		if err != nil {
			dbSetupErr <- err
		}
	}()

	testResults := make(chan error, 1)
	go func() {
		testResults <- s.mgr.Test(ctx, run.TestParams{
			AppRoot:     req.AppRoot,
			WorkingDir:  req.WorkingDir,
			DBClusterID: clusterID,
			Args:        req.Args,
			Stdout:      &streamWriter{stream: stream, stderr: false},
			Stderr:      &streamWriter{stream: stream, stderr: true},
		})
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

// Check checks the app for compilation errors.
func (s *Server) Check(req *daemonpb.CheckRequest, stream daemonpb.Daemon_CheckServer) error {
	log := newStreamLogger(stream)
	err := s.mgr.Check(stream.Context(), req.AppRoot, req.WorkingDir)
	if err != nil {
		log.Error().Msg(err.Error())
		streamExit(stream, 1)
	} else {
		streamExit(stream, 0)
	}
	return nil
}

// OnStart implements run.EventListener.
func (s *Server) OnStart(r *run.Run) {}

// OnStop implements run.EventListener.
func (s *Server) OnStop(r *run.Run) {}

// OnStdout implements run.EventListener.
func (s *Server) OnStdout(r *run.Run, line []byte) {
	s.mu.Lock()
	stream, ok := s.streams[r.ID]
	s.mu.Unlock()

	if ok {
		stream.Send(&daemonpb.RunMessage{Msg: &daemonpb.RunMessage_Output{
			Output: &daemonpb.CommandOutput{Stdout: line},
		}})
	}
}

// OnStderr implements run.EventListener.
func (s *Server) OnStderr(r *run.Run, line []byte) {
	s.mu.Lock()
	stream, ok := s.streams[r.ID]
	s.mu.Unlock()

	if ok {
		stream.Send(&daemonpb.RunMessage{Msg: &daemonpb.RunMessage_Output{
			Output: &daemonpb.CommandOutput{Stderr: line},
		}})
	}
}

// parseApp parses the app.
func (s *Server) parseApp(appRoot, workingDir string, parseTests bool) (*parser.Result, error) {
	modPath := filepath.Join(appRoot, "go.mod")
	modData, err := ioutil.ReadFile(modPath)
	if err != nil {
		return nil, err
	}
	mod, err := modfile.Parse(modPath, modData, nil)
	if err != nil {
		return nil, err
	}

	cfg := &parser.Config{
		AppRoot:    appRoot,
		Version:    "",
		ModulePath: mod.Module.Mod.Path,
		WorkingDir: workingDir,
		ParseTests: parseTests,
	}
	return parser.Parse(cfg)
}

func requiresSQLDB(md *meta.Data) bool {
	for _, svc := range md.Svcs {
		if len(svc.Migrations) > 0 {
			return true
		}
	}
	return false
}
