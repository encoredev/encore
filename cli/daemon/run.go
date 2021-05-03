package daemon

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"time"

	"encr.dev/cli/daemon/internal/appfile"
	"encr.dev/cli/daemon/internal/manifest"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/cli/internal/onboarding"
	"encr.dev/cli/internal/version"
	"encr.dev/parser"
	daemonpb "encr.dev/proto/encore/daemon"
	meta "encr.dev/proto/encore/parser/meta/v1"
	"github.com/logrusorgru/aurora/v3"
	"github.com/rs/zerolog/log"
	"golang.org/x/mod/modfile"
)

// Run runs the application.
func (s *Server) Run(req *daemonpb.RunRequest, stream daemonpb.Daemon_RunServer) error {
	slog := &streamLog{stream: stream}
	stdout := slog.Stdout()
	stderr := slog.Stderr()

	sendErr := func(err error) {
		stderr.Write([]byte(err.Error() + "\n"))
		stream.Send(&daemonpb.CommandMessage{
			Msg: &daemonpb.CommandMessage_Exit{Exit: &daemonpb.CommandExit{
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
		if _, err := exec.LookPath("docker"); err != nil {
			sendErr(fmt.Errorf("This application requires docker to run since it uses an SQL database. Install docker first."))
			return nil
		}

		if err := cluster.Start(slog); err != nil {
			sendErr(fmt.Errorf("Database setup failed: %v", err))
			return nil
		}

		// Set up the database asynchronously since it can take a while.
		go func() {
			if err := cluster.CreateAndMigrate(stream.Context(), req.AppRoot, parse.Meta); err != nil {
				dbSetupErr <- err
			}
		}()
	}

	// Check for available update before we start the proc
	// so the output from the proc doesn't race with our
	// prints below.
	newVer := s.availableUpdate()

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
	s.streams[run.ID] = slog
	s.mu.Unlock()

	if newVer != "" {
		stdout.Write([]byte(aurora.Sprintf(
			aurora.Yellow("New Encore release available: %s (you have %s)\nUpdate with: encore version update\n\n"),
			newVer, version.Version)))
	}

	pid := run.Proc().Pid
	stdout.Write([]byte(fmt.Sprintf("API Base URL:      http://localhost:%d\n", run.Port)))
	stdout.Write([]byte(fmt.Sprintf("Dev Dashboard URL: http://localhost:%d/%s\n", s.mgr.DashPort, man.AppID)))
	if req.Debug {
		stdout.Write([]byte(fmt.Sprintf("Process ID:        %d\n", pid)))
	}

	go func() {
		// Wait a little bit for the app to start
		select {
		case <-run.Done():
			return
		case <-time.After(5 * time.Second):
			showFirstRunExperience(run, parse.Meta, stdout)
		}
	}()

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
	slog := &streamLog{stream: stream}
	sendErr := func(err error) {
		slog.Stderr().Write([]byte(err.Error() + "\n"))
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

	setupCtx, setupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	clusterID := man.AppID + "-test"
	cluster := s.cm.Init(setupCtx, &sqldb.InitParams{
		ClusterID: clusterID,
		Memfs:     true,
		Meta:      parse.Meta,
	})

	// Set up the database asynchronously since it can take a while.
	dbSetupErr := make(chan error, 1)
	go func() {
		defer setupCancel()
		if err := cluster.Start(&streamLog{stream: stream}); err != nil {
			dbSetupErr <- err
		} else if err := cluster.Recreate(setupCtx, req.AppRoot, nil, parse.Meta); err != nil {
			dbSetupErr <- err
		}
	}()

	testCtx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	testResults := make(chan error, 1)
	go func() {
		testResults <- s.mgr.Test(testCtx, run.TestParams{
			AppRoot:     req.AppRoot,
			WorkingDir:  req.WorkingDir,
			DBClusterID: clusterID,
			Args:        req.Args,
			Stdout:      slog.Stdout(),
			Stderr:      slog.Stderr(),
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
	slog := &streamLog{stream: stream}
	log := newStreamLogger(slog)
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

// OnReload implements run.EventListener.
func (s *Server) OnReload(r *run.Run) {}

// OnStop implements run.EventListener.
func (s *Server) OnStop(r *run.Run) {}

// OnStdout implements run.EventListener.
func (s *Server) OnStdout(r *run.Run, line []byte) {
	s.mu.Lock()
	slog, ok := s.streams[r.ID]
	s.mu.Unlock()

	if ok {
		slog.Stdout().Write(line)
	}
}

// OnStderr implements run.EventListener.
func (s *Server) OnStderr(r *run.Run, line []byte) {
	s.mu.Lock()
	slog, ok := s.streams[r.ID]
	s.mu.Unlock()

	if ok {
		slog.Stderr().Write(line)
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

func showFirstRunExperience(run *run.Run, md *meta.Data, stdout io.Writer) {
	if state, err := onboarding.Load(); err == nil {
		if !state.FirstRun.IsSet() {
			// Is there a suitable endpoint to call?
			var rpc *meta.RPC
			var payload []byte
			for _, svc := range md.Svcs {
				for _, r := range svc.Rpcs {
					if s := genSchema(md, r.RequestSchema); rpc == nil || len(s) < len(payload) {
						rpc = r
						payload = s
					}
				}
			}
			if rpc != nil {
				state.FirstRun.Set()
				if err := state.Write(); err == nil {
					payloadArg := ""
					if len(payload) > 0 {
						payloadArg = fmt.Sprintf(" -d '%s'", payload)
					}
					stdout.Write([]byte(aurora.Sprintf("\nHint: make an API call by running: %s\n",
						aurora.Cyan(fmt.Sprintf("curl http://localhost:%d/%s.%s%s", run.Port, rpc.ServiceName, rpc.Name, payloadArg)))))
				}
			}
		}
	}
}
