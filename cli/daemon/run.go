package daemon

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/logrusorgru/aurora/v3"

	"encr.dev/cli/daemon/run"
	"encr.dev/internal/optracker"
	"encr.dev/internal/version"
	daemonpb "encr.dev/proto/encore/daemon"
)

// Run runs the application.
func (s *Server) Run(req *daemonpb.RunRequest, stream daemonpb.Daemon_RunServer) error {
	ctx := stream.Context()
	slog := &streamLog{stream: stream, buffered: true}
	stderr := slog.Stderr(false)

	sendExit := func(code int32) {
		stream.Send(&daemonpb.CommandMessage{
			Msg: &daemonpb.CommandMessage_Exit{Exit: &daemonpb.CommandExit{
				Code: code,
			}},
		})
	}

	// ListenAddr should always be passed but guard against old clients.
	listenAddr := req.ListenAddr
	if listenAddr == "" {
		listenAddr = ":4000"
	}
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		if errIsAddrInUse(err) {
			fmt.Fprintln(stderr, aurora.Sprintf(aurora.Red("Failed to run on %s - port is already in use"), listenAddr))
		} else {
			fmt.Fprintln(stderr, aurora.Sprintf(aurora.Red("Failed to run on %s - %v"), listenAddr, err))
		}

		if host, port, ok := findAvailableAddr(listenAddr); ok {
			if host == "localhost" || host == "127.0.0.1" {
				fmt.Fprintf(stderr, "Note: port %d is available; specify %s to use it\n",
					port, aurora.Sprintf(aurora.Cyan("--port=%d"), port))
			} else {
				fmt.Fprintf(stderr, "Note: address %s:%d is available; specify %s to use it\n",
					host, port, aurora.Sprintf(aurora.Cyan("--listen=%s:%d"), host, port))
			}
		} else {
			fmt.Fprintf(stderr, "Note: specify %s to run on another port\n",
				aurora.Cyan("--port=NUMBER"))
		}

		sendExit(1)
		return nil
	}
	defer ln.Close()

	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		fmt.Fprintln(stderr, aurora.Sprintf(aurora.Red("failed to resolve app: %v"), err))
		sendExit(1)
		return nil
	}

	ops := optracker.New(stderr, stream)

	// Check for available update before we start the proc
	// so the output from the proc doesn't race with our
	// prints below.
	newVer := s.availableUpdate()

	// If force upgrade has been enabled, we force the upgrade now before we try and run the app
	if newVer != nil && newVer.ForceUpgrade {
		_, _ = fmt.Fprintf(stderr, aurora.Red("An urgent security update for Encore is available.").String()+"\n")
		if newVer.SecurityNotes != "" {
			_, _ = fmt.Fprintf(stderr, aurora.Sprintf(aurora.Yellow("%s"), newVer.SecurityNotes)+"\n")
		}

		_, _ = fmt.Fprintf(stderr, "Upgrading Encore to %v...\n", newVer.Version())
		if err := newVer.DoUpgrade(stderr, stderr); err != nil {
			_, _ = fmt.Fprintf(stderr, aurora.Sprintf(aurora.Red("Upgrade failed: %v"), err)+"\n")
		}

		slog.FlushBuffers()
		sendExit(1) // Kill the client
		os.Exit(1)  // Kill the daemon too
		return nil
	}

	// Hold the stream mutex so we can set up the stream map
	// before output starts.
	s.mu.Lock()

	// If the listen addr contains no interface, render it as "localhost:port"
	// instead of just ":port".
	displayListenAddr := req.ListenAddr
	if strings.HasPrefix(listenAddr, ":") {
		displayListenAddr = "localhost" + req.ListenAddr
	}

	run, err := s.mgr.Start(ctx, run.StartParams{
		App:        app,
		WorkingDir: req.WorkingDir,
		Listener:   ln,
		ListenAddr: displayListenAddr,
		Watch:      req.Watch,
		Environ:    req.Environ,
		OpsTracker: ops,
		Debug:      req.Debug,
	})
	if err != nil {
		s.mu.Unlock()
		sendExit(1)
		return nil
	}
	defer run.ResourceServers.StopAll()
	s.streams[run.ID] = slog
	s.mu.Unlock()

	ops.AllDone()

	stderr.Write([]byte("\n"))
	pid := run.Proc().Pid
	fmt.Fprintf(stderr, "  Encore development server running!\n\n")

	fmt.Fprintf(stderr, "  Your API is running at:     %s\n", aurora.Cyan("http://"+run.ListenAddr))
	fmt.Fprintf(stderr, "  Development Dashboard URL:  %s\n", aurora.Cyan(fmt.Sprintf(
		"http://localhost:%d/%s", s.mgr.DashPort, app.PlatformOrLocalID())))
	if req.Debug {
		fmt.Fprintf(stderr, "  Process ID:             %d\n", aurora.Cyan(pid))
	}
	// Log which experiments are enabled, if any
	if exp := run.Proc().Experiments.List(); len(exp) > 0 {
		strs := make([]string, len(exp))
		for i, e := range exp {
			strs[i] = string(e)
		}
		fmt.Fprintf(stderr, "  Enabled experiment(s):      %s\n", aurora.Yellow(strings.Join(strs, ", ")))
	}

	// If there's a newer version available, print a message.
	if newVer != nil {
		if newVer.SecurityUpdate {
			stderr.Write([]byte(aurora.Sprintf(
				aurora.Yellow("\n  New Encore release available with security updates: %s (you have %s)\n  Update with: encore version update\n"),
				newVer.Version(), version.Version)))

			if newVer.SecurityNotes != "" {
				stderr.Write([]byte(aurora.Sprintf(
					aurora.Faint("\n  %s\n"),
					newVer.SecurityNotes)))
			}
		} else {
			stderr.Write([]byte(aurora.Sprintf(
				aurora.Faint("\n  New Encore release available: %s (you have %s)\n  Update with: encore version update\n"),
				newVer.Version(), version.Version)))
		}
	}
	stderr.Write([]byte("\n"))

	slog.FlushBuffers()

	go func() {
		// Wait a little bit for the app to start
		select {
		case <-run.Done():
			return
		case <-time.After(5 * time.Second):
			parse, err := s.parseApp(req.AppRoot, req.WorkingDir, false)
			if err == nil {
				showFirstRunExperience(run, parse.Meta, stderr)
			}
		}
	}()

	<-run.Done() // wait for run to complete

	s.mu.Lock()
	delete(s.streams, run.ID)
	s.mu.Unlock()
	return nil
}
