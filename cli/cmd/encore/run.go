package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"

	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"
	"encr.dev/cli/internal/onboarding"
	daemonpb "encr.dev/proto/encore/daemon"
)

var (
	color   bool
	noColor bool // for "--no-color" compatibility
	debug   = cmdutil.Oneof{
		Value:       "",
		NoOptDefVal: "enabled",
		Allowed:     []string{"enabled", "break"},
		Flag:        "debug",
		FlagShort:   "", // no short flag
		Desc:        "Compile for debugging (disables some optimizations)",
		TypeDesc:    "string",
	}
	watch    bool
	listen   string
	port     uint
	jsonLogs bool
	browser  = cmdutil.Oneof{
		Value:     "auto",
		Allowed:   []string{"auto", "never", "always"},
		Flag:      "browser",
		FlagShort: "", // no short flag
		Desc:      "Whether to open the local development dashboard in the browser on startup",
		TypeDesc:  "string",
	}
)

func init() {
	runCmd := &cobra.Command{
		Use:   "run [--debug] [--watch=true] [--port=4000] [--listen=<listen-addr>]",
		Short: "Runs your application",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			appRoot, wd := determineAppRoot()
			// If the user didn't explicitly set --watch and we're in debug mode, disable watching
			// as we typically don't want to swap the process when the user is debugging.
			if !cmd.Flag("watch").Changed && debug.Value != "" {
				watch = false
			}
			runApp(appRoot, wd)
		},
	}

	isTerm := term.IsTerminal(int(os.Stdout.Fd()))

	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch for changes and live-reload")
	runCmd.Flags().StringVar(&listen, "listen", "", "Address to listen on (for example \"0.0.0.0:4000\")")
	runCmd.Flags().UintVarP(&port, "port", "p", 4000, "Port to listen on")
	runCmd.Flags().BoolVar(&jsonLogs, "json", false, "Display logs in JSON format")
	runCmd.Flags().StringVarP(&nsName, "namespace", "n", "", "Namespace to use (defaults to active namespace)")
	runCmd.Flags().BoolVar(&color, "color", isTerm, "Whether to display colorized output")
	runCmd.Flags().BoolVar(&noColor, "no-color", false, "Equivalent to --color=false")
	runCmd.Flags().MarkHidden("no-color")
	debug.AddFlag(runCmd)
	browser.AddFlag(runCmd)
}

// runApp runs the app.
func runApp(appRoot, wd string) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

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

	browserMode := daemonpb.RunRequest_BROWSER_AUTO
	switch browser.Value {
	case "auto":
		browserMode = daemonpb.RunRequest_BROWSER_AUTO
	case "never":
		browserMode = daemonpb.RunRequest_BROWSER_NEVER
	case "always":
		browserMode = daemonpb.RunRequest_BROWSER_ALWAYS
	}

	debugMode := daemonpb.RunRequest_DEBUG_DISABLED
	switch debug.Value {
	case "enabled":
		debugMode = daemonpb.RunRequest_DEBUG_ENABLED
	case "break":
		debugMode = daemonpb.RunRequest_DEBUG_BREAK
	}

	daemon := setupDaemon(ctx)
	stream, err := daemon.Run(ctx, &daemonpb.RunRequest{
		AppRoot:    appRoot,
		DebugMode:  debugMode,
		Watch:      watch,
		WorkingDir: wd,
		ListenAddr: listenAddr,
		Environ:    os.Environ(),
		TraceFile:  root.TraceFile,
		Namespace:  nonZeroPtr(nsName),
		Browser:    browserMode,
	})
	if err != nil {
		fatal(err)
	}

	cmdutil.ClearTerminalExceptFirstNLines(1)

	var converter cmdutil.OutputConverter
	if !jsonLogs {
		converter = cmdutil.ConvertJSONLogs(cmdutil.Colorize(color && !noColor))
	}
	code := cmdutil.StreamCommandOutput(stream, converter)
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

func init() {
}
