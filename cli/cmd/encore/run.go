package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"

	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	"encr.dev/cli/cmd/encore/root"
	"encr.dev/cli/internal/onboarding"
	"encr.dev/pkg/ansi"
	daemonpb "encr.dev/proto/encore/daemon"
)

var (
	tunnel bool
	debug  bool
	watch  bool
	listen string
	port   uint
)

var runCmd = &cobra.Command{
	Use:   "run [--debug] [--watch=true] [--port=4000] [--listen=<listen-addr>]",
	Short: "Runs your application",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		appRoot, wd := determineAppRoot()
		runApp(appRoot, wd)
	},
}

// runApp runs the app.
func runApp(appRoot, wd string) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-interrupt
		cancel()
	}()

	// Determine listen addr.
	var listenAddr string

	if listen == "" {
		// If we have no listen address at all, listen on localhost.
		// (we do this so MacOS's firewall doesn't ask for permission for the daemon to listen on all interfaces)
		listenAddr = fmt.Sprintf("127.0.0.1:%d", port)
	} else if _, _, err := net.SplitHostPort(listen); err == nil {
		// If --listen is given with a port, use that directly and ignore --port.
		listenAddr = listen
	} else {
		// Otherwise use --listen as the host and --port as the port.
		listenAddr = net.JoinHostPort(listen, strconv.Itoa(int(port)))
	}

	daemon := setupDaemon(ctx)
	stream, err := daemon.Run(ctx, &daemonpb.RunRequest{
		AppRoot:    appRoot,
		Tunnel:     tunnel,
		Debug:      debug,
		Watch:      watch,
		WorkingDir: wd,
		ListenAddr: listenAddr,
		Environ:    os.Environ(),
		TraceFile:  root.TraceFile,
		Namespace:  nonZeroPtr(nsName),
	})
	if err != nil {
		fatal(err)
	}

	clearTerminalExceptFirstLine()
	code := streamCommandOutput(stream, convertJSONLogs())
	if code == 0 {
		if state, err := onboarding.Load(); err == nil {
			if state.DeployHint.Set() {
				if err := state.Write(); err == nil {
					_, _ = fmt.Println(aurora.Sprintf("\nHint: deploy your app to the cloud by running: %s", aurora.Cyan("git push encore")))
				}
			}
		}
	}
	os.Exit(code)
}

func clearTerminalExceptFirstLine() {
	// Clear the screen except for the first line.
	if _, height, err := terminal.GetSize(int(os.Stdout.Fd())); err == nil {
		count := height - 2
		if count > 0 {
			_, _ = os.Stdout.Write(bytes.Repeat([]byte{'\n'}, count))
		}
		_, _ = fmt.Fprint(os.Stdout, ansi.SetCursorPosition(2, 1)+ansi.ClearScreen(ansi.CursorToBottom))
	}
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVar(&tunnel, "tunnel", false, "Create a tunnel to your machine for others to test against")
	runCmd.Flags().BoolVar(&debug, "debug", false, "Compile for debugging (disables some optimizations)")
	runCmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch for changes and live-reload")
	runCmd.Flags().StringVar(&listen, "listen", "", "Address to listen on (for example \"0.0.0.0:4000\")")
	runCmd.Flags().UintVarP(&port, "port", "p", 4000, "Port to listen on")
	runCmd.Flags().StringVarP(&nsName, "namespace", "n", "", "Namespace to use (defaults to active namespace)")
}
