package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"

	"encr.dev/cli/daemon/run"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/errlist"
	"encr.dev/pkg/fns"
	daemonpb "encr.dev/proto/encore/daemon"
)

// RunSpec boots the application on a random local port, waits for
// /__encore/healthz, then runs each spec command against it and streams the
// results. Designed for non-interactive agents.
func (s *Server) RunSpec(req *daemonpb.RunSpecRequest, stream daemonpb.Daemon_RunSpecServer) error {
	ctx := stream.Context()
	sink := newRunSpecSink(stream)
	stderr := sink.Stderr(false)

	total := int32(len(req.Commands))
	sendComplete := func(succeeded int32, errMsg string) {
		_ = stream.Send(&daemonpb.RunSpecMessage{Msg: &daemonpb.RunSpecMessage_Complete{
			Complete: &daemonpb.SpecComplete{
				Succeeded: succeeded,
				Total:     total,
				Error:     errMsg,
			},
		}})
	}

	// Validate up front so callers get a fast error before we spend time
	// compiling the app. The CLI also validates, but the daemon is the
	// authoritative boundary. An empty command list is allowed — it makes
	// `encore check` with no spec a useful "does this app boot?" smoke test.
	for i, c := range req.Commands {
		curl := c.GetCurl()
		if curl == nil {
			sendComplete(0, fmt.Sprintf("command %d: unsupported command (only curl is supported)", i+1))
			return nil
		}
		if !strings.HasPrefix(curl.Path, "/") {
			sendComplete(0, fmt.Sprintf("command %d: path %q must be relative (start with '/')", i+1, curl.Path))
			return nil
		}
	}

	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		sendComplete(0, fmt.Sprintf("failed to resolve app: %v", err))
		return nil
	}

	ns, err := s.namespaceOrActive(ctx, app, req.Namespace)
	if err != nil {
		sendComplete(0, fmt.Sprintf("failed to resolve namespace: %v", err))
		return nil
	}

	// Pre-bind a free local port and pass the listener to the manager so
	// there is no race window between picking a port and the app binding to it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		sendComplete(0, fmt.Sprintf("failed to allocate local port: %v", err))
		return nil
	}
	// On the happy path mgr.Start takes ownership of ln and closes it on
	// teardown. If Start returns an error we close it ourselves below.
	listenerOwnedByMgr := false
	defer func() {
		if !listenerOwnedByMgr {
			fns.CloseIgnore(ln)
		}
	}()
	listenAddr := ln.Addr().String()

	// Use a line-mode op tracker so the compile/start phases emit one progress
	// line each to the client's stderr. No ANSI, no spinner — safe for agents.
	ops := optracker.NewLineMode(stderr)
	defer ops.AllDone()

	runInstance, err := s.mgr.Start(ctx, run.StartParams{
		App:        app,
		NS:         ns,
		WorkingDir: req.WorkingDir,
		Listener:   ln,
		ListenAddr: listenAddr,
		Watch:      false,
		Environ:    req.Environ,
		OpsTracker: ops,
		Browser:    run.BrowserModeNever,
	})
	if err != nil {
		// Forward errlist details (compile errors, parse errors) as plain text
		// to stderr so the client can render them.
		if errList := run.AsErrorList(err); errList != nil {
			_, _ = io.WriteString(stderr, errList.Error())
			if !strings.HasSuffix(errList.Error(), "\n") {
				_, _ = io.WriteString(stderr, "\n")
			}
			sendComplete(0, "failed to start app (see stderr for details)")
		} else {
			sendComplete(0, fmt.Sprintf("failed to start app: %v", err))
		}
		return nil
	}
	listenerOwnedByMgr = true
	defer func() {
		runInstance.Close()
		fmt.Fprintln(stderr, "encore: app shut down")
	}()

	// Register the sink so OnStdout/OnStderr from run.EventListener forward
	// the app's log output to our stream as CommandOutput messages.
	s.mu.Lock()
	s.streams[runInstance.ID] = sink
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.streams, runInstance.ID)
		s.mu.Unlock()
	}()

	timeout := time.Duration(req.ReadyTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	healthzURL := "http://" + runInstance.ListenAddr + "/__encore/healthz"
	fmt.Fprintf(stderr, "encore: waiting for app to start (timeout %s)...\n", timeout)
	if err := waitForHealthz(ctx, healthzURL, timeout); err != nil {
		sendComplete(0, fmt.Sprintf("timed out waiting for /__encore/healthz: %v", err))
		return nil
	}
	fmt.Fprintln(stderr, "encore: app is ready")

	var succeeded int32
	for i, c := range req.Commands {
		// Stream cancellation (client disconnect) should stop further commands.
		if err := ctx.Err(); err != nil {
			sendComplete(succeeded, fmt.Sprintf("interrupted: %v", err))
			return nil
		}
		curl := c.GetCurl()
		display := formatCurlDisplay(curl)
		stdoutBytes, stderrBytes, exitCode := runCurl(ctx, runInstance.ListenAddr, curl)
		if exitCode == 0 {
			succeeded++
		}
		_ = stream.Send(&daemonpb.RunSpecMessage{Msg: &daemonpb.RunSpecMessage_Result{
			Result: &daemonpb.SpecCommandResult{
				Index:    int32(i + 1),
				Total:    total,
				Display:  display,
				Stdout:   stdoutBytes,
				Stderr:   stderrBytes,
				ExitCode: exitCode,
			},
		}})
	}

	sendComplete(succeeded, "")
	return nil
}

// runCurl executes a single curl command against the app and returns its
// stdout, stderr, and exit code. The URL is appended after user-supplied args
// so callers can't accidentally override it via a positional URL.
func runCurl(ctx context.Context, listenAddr string, c *daemonpb.CurlCommand) (stdout, stderr []byte, exitCode int32) {
	argv := append([]string{"-s"}, c.Args...)
	argv = append(argv, "http://"+listenAddr+c.Path)

	cmd := exec.CommandContext(ctx, "curl", argv...)
	var sout, serr bytes.Buffer
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	err := cmd.Run()
	if err == nil {
		return sout.Bytes(), serr.Bytes(), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return sout.Bytes(), serr.Bytes(), int32(exitErr.ExitCode())
	}
	// curl could not be exec'd at all (e.g. not on PATH).
	if serr.Len() > 0 {
		serr.WriteByte('\n')
	}
	serr.WriteString(err.Error())
	return sout.Bytes(), serr.Bytes(), -1
}

func formatCurlDisplay(c *daemonpb.CurlCommand) string {
	parts := append([]string{"curl", c.Path}, c.Args...)
	return strings.Join(parts, " ")
}

// waitForHealthz polls /__encore/healthz with exponential backoff until it
// returns 200, the context is canceled, or the timeout elapses.
func waitForHealthz(ctx context.Context, url string, timeout time.Duration) error {
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 50 * time.Millisecond
	b.MaxInterval = 250 * time.Millisecond
	b.MaxElapsedTime = timeout

	client := &http.Client{Timeout: 2 * time.Second}
	op := func() error {
		req, err := http.NewRequestWithContext(pollCtx, "GET", url, nil)
		if err != nil {
			return backoff.Permanent(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		fns.CloseIgnore(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return nil
	}
	return backoff.Retry(op, backoff.WithContext(b, pollCtx))
}

// runSpecSink adapts a Daemon_RunSpecServer to the runStreamSink interface so
// the daemon's run.EventListener can forward the app's stdout/stderr output to
// the gRPC client as CommandOutput messages.
type runSpecSink struct {
	stream daemonpb.Daemon_RunSpecServer
	mu     sync.Mutex
}

func newRunSpecSink(stream daemonpb.Daemon_RunSpecServer) *runSpecSink {
	return &runSpecSink{stream: stream}
}

func (s *runSpecSink) Stdout(_ bool) io.Writer { return runSpecWriter{sink: s, stderr: false} }
func (s *runSpecSink) Stderr(_ bool) io.Writer { return runSpecWriter{sink: s, stderr: true} }

func (s *runSpecSink) Error(err *errlist.List) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil || len(err.List) == 0 {
		return
	}
	out := &daemonpb.CommandOutput{Stderr: []byte(err.Error() + "\n")}
	_ = s.stream.Send(&daemonpb.RunSpecMessage{Msg: &daemonpb.RunSpecMessage_Output{Output: out}})
}

type runSpecWriter struct {
	sink   *runSpecSink
	stderr bool
}

func (w runSpecWriter) Write(b []byte) (int, error) {
	w.sink.mu.Lock()
	defer w.sink.mu.Unlock()
	out := &daemonpb.CommandOutput{}
	if w.stderr {
		out.Stderr = b
	} else {
		out.Stdout = b
	}
	if err := w.sink.stream.Send(&daemonpb.RunSpecMessage{Msg: &daemonpb.RunSpecMessage_Output{Output: out}}); err != nil {
		return 0, err
	}
	return len(b), nil
}
