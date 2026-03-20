package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/mod/modfile"

	"encr.dev/cli/daemon/run"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/appfile"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/paths"
	daemonpb "encr.dev/proto/encore/daemon"
)

// ExecScript executes a one-off script.
func (s *Server) ExecScript(req *daemonpb.ExecScriptRequest, stream daemonpb.Daemon_ExecScriptServer) error {
	ctx := stream.Context()
	slog := &streamLog{stream: stream, buffered: true}
	stderr := slog.Stderr(false)
	sendErr := func(err error) {
		if list := run.AsErrorList(err); list != nil {
			_ = list.SendToStream(stream)
		} else {
			errStr := err.Error()
			if !strings.HasSuffix(errStr, "\n") {
				errStr += "\n"
			}
			slog.Stderr(false).Write([]byte(errStr))
		}
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

	ns, err := s.namespaceOrActive(ctx, app, req.Namespace)
	if err != nil {
		sendErr(err)
		return nil
	}

	ops := optracker.New(stderr, stream)
	defer ops.AllDone() // Kill the tracker when we exit this function

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

	// Note: TypeScript apps use the ExecSpec RPC instead, which allows
	// the CLI to run the command locally with stdin support.
	if app.Lang() != appfile.LangGo {
		sendErr(fmt.Errorf("unsupported language for ExecScript: %s", app.Lang()))
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

	commandRelPath := filepath.ToSlash(filepath.Join(req.WorkingDir, req.ScriptArgs[0]))
	scriptArgs := req.ScriptArgs[1:]
	commandPkg := paths.Pkg(mod.Module.Mod.Path).JoinSlash(paths.RelSlash(commandRelPath))

	p := run.ExecScriptParams{
		App:        app,
		NS:         ns,
		WorkingDir: req.WorkingDir,
		Environ:    req.Environ,
		MainPkg:    commandPkg,
		ScriptArgs: scriptArgs,
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

// ExecSpec returns the specification for how to run an exec command,
// allowing the CLI to execute it directly with stdin support.
// It streams progress messages during setup, then sends the spec as the final message.
func (s *Server) ExecSpec(req *daemonpb.ExecSpecRequest, stream daemonpb.Daemon_ExecSpecServer) error {
	ctx := stream.Context()
	// Wrap the ExecSpec stream so it can be used with streamLog and optracker,
	// which expect a commandStream (Send(*CommandMessage)).
	adapter := &execSpecStreamAdapter{stream: stream}
	slog := &streamLog{stream: adapter, buffered: true}
	stderr := slog.Stderr(false)
	sendErr := func(err error) {
		if list := run.AsErrorList(err); list != nil {
			_ = list.SendToStream(adapter)
		} else {
			errStr := err.Error()
			if !strings.HasSuffix(errStr, "\n") {
				errStr += "\n"
			}
			slog.Stderr(false).Write([]byte(errStr))
		}
	}

	ctx, tracer, err := s.beginTracing(ctx, req.AppRoot, req.WorkingDir, nil)
	if err != nil {
		sendErr(err)
		return nil
	}
	defer fns.CloseIgnore(tracer)

	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		sendErr(err)
		return nil
	}

	if app.Lang() != appfile.LangTS {
		sendErr(errors.New("exec spec is only supported for TypeScript apps"))
		return nil
	}

	ns, err := s.namespaceOrActive(ctx, app, req.Namespace)
	if err != nil {
		sendErr(err)
		return nil
	}

	ops := optracker.New(stderr, adapter)
	defer ops.AllDone()

	defer func() {
		if recovered := recover(); recovered != nil {
			var panicErr error
			switch recovered := recovered.(type) {
			case error:
				panicErr = recovered
			default:
				panicErr = fmt.Errorf("%+v", recovered)
			}
			stack := debug.Stack()
			log.Err(panicErr).Msgf("panic during exec spec:\n%s", stack)
			sendErr(fmt.Errorf("panic during exec spec: %v", panicErr))
		}
	}()

	spec, err := s.mgr.ExecSpec(ctx, run.ExecSpecParams{
		App:        app,
		NS:         ns,
		WorkingDir: req.WorkingDir,
		Environ:    req.Environ,
		Command:    req.ScriptArgs[0],
		ScriptArgs: req.ScriptArgs[1:],
		TempDir:    req.TempDir,
		OpTracker:  ops,
	})
	if err != nil {
		sendErr(err)
		return nil
	}

	// Send the spec as the final message.
	return stream.Send(&daemonpb.ExecSpecMessage{
		Msg: &daemonpb.ExecSpecMessage_Spec{
			Spec: &daemonpb.ExecSpecResponse{
				Command: spec.Command,
				Args:    spec.Args,
				Environ: spec.Environ,
			},
		},
	})
}

// execSpecStreamAdapter adapts a Daemon_ExecSpecServer stream to the
// commandStream interface, wrapping CommandOutput in ExecSpecMessage.
type execSpecStreamAdapter struct {
	stream daemonpb.Daemon_ExecSpecServer
}

func (a *execSpecStreamAdapter) Send(msg *daemonpb.CommandMessage) error {
	switch m := msg.Msg.(type) {
	case *daemonpb.CommandMessage_Output:
		return a.stream.Send(&daemonpb.ExecSpecMessage{
			Msg: &daemonpb.ExecSpecMessage_Output{Output: m.Output},
		})
	case *daemonpb.CommandMessage_Errors:
		// Send structured errors as stderr output so the client can display them.
		return a.stream.Send(&daemonpb.ExecSpecMessage{
			Msg: &daemonpb.ExecSpecMessage_Output{Output: &daemonpb.CommandOutput{
				Stderr: m.Errors.Errinsrc,
			}},
		})
	default:
		return nil
	}
}
