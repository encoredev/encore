package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"golang.org/x/mod/modfile"

	"encr.dev/cli/daemon/run"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/paths"
	daemonpb "encr.dev/proto/encore/daemon"
)

// ExecScript executes a one-off script.
func (s *Server) ExecScript(req *daemonpb.ExecScriptRequest, stream daemonpb.Daemon_ExecScriptServer) error {
	ctx := stream.Context()
	slog := &streamLog{stream: stream, buffered: true}
	stderr := slog.Stderr(false)
	sendErr := func(err error) {
		slog.Stderr(false).Write([]byte(err.Error() + "\n"))
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

	modPath := filepath.Join(app.Root(), "go.mod")
	modData, err := os.ReadFile(modPath)
	if err != nil {
		sendErr(err)
		return nil
	}
	mod, err := modfile.Parse(modPath, modData, nil)
	if err != nil {
		sendErr(err)
		return nil
	}

	commandPkg := paths.Pkg(mod.Module.Mod.Path).JoinSlash(paths.RelSlash(req.CommandRelPath))

	ops := optracker.New(stderr, stream)

	testResults := make(chan error, 1)
	defer func() {
		if recovered := recover(); recovered != nil {
			var err error
			switch recovered := recovered.(type) {
			case error:
				err = recovered
			default:
				err = fmt.Errorf("%v", recovered)
			}
			log.Err(err).Msg("panic during script execution")
			testResults <- fmt.Errorf("panic occured within Encore during script execution: %v\n", recovered)
		}
	}()

	p := run.ExecScriptParams{
		App:        app,
		WorkingDir: req.WorkingDir,
		Environ:    req.Environ,
		MainPkg:    commandPkg,
		ScriptArgs: req.ScriptArgs,
		Stdout:     slog.Stdout(false),
		Stderr:     slog.Stderr(false),
		OpTracker:  ops,
	}
	if err := s.mgr.ExecScript(stream.Context(), p); err != nil {
		sendErr(err)
	} else {
		streamExit(stream, 0)
	}
	return nil
}
