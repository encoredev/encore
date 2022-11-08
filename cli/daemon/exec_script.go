package daemon

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"encr.dev/cli/daemon/run"
	"encr.dev/cli/internal/appfile"
	"encr.dev/internal/optracker"
	daemonpb "encr.dev/proto/encore/daemon"
)

// ExecScript executes a one-off script.
func (s *Server) ExecScript(req *daemonpb.ExecScriptRequest, stream daemonpb.Daemon_ExecScriptServer) error {
	slog := &streamLog{stream: stream, buffered: true}
	stderr := slog.Stderr(false)
	sendErr := func(err error) {
		slog.Stderr(false).Write([]byte(err.Error() + "\n"))
		streamExit(stream, 1)
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

	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		sendErr(err)
		return nil
	}

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
		App:           app,
		WorkingDir:    req.WorkingDir,
		Environ:       req.Environ,
		ScriptRelPath: req.ScriptRelPath,
		ScriptArgs:    req.ScriptArgs,
		Parse:         parse,
		Stdout:        slog.Stdout(false),
		Stderr:        slog.Stderr(false),
		OpTracker:     ops,
	}
	if err := s.mgr.ExecScript(stream.Context(), p); err != nil {
		sendErr(err)
	} else {
		streamExit(stream, 0)
	}
	return nil
}
