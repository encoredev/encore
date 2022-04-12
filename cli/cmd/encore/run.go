package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/cobra"

	"encr.dev/cli/internal/onboarding"
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
	// If --listen is given with a port, use that directly and ignore --port.
	var listenAddr string
	if _, _, err := net.SplitHostPort(listen); err == nil {
		listenAddr = listen
	} else {
		listenAddr = fmt.Sprintf("%s:%d", listen, port)
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
	})
	if err != nil {
		fatal(err)
	}

	code := streamCommandOutput(stream)
	if code == 0 {
		if state, err := onboarding.Load(); err == nil {
			if state.DeployHint.Set() {
				if err := state.Write(); err == nil {
					fmt.Println(aurora.Sprintf("\nHint: deploy your app to the cloud by running: %s", aurora.Cyan("git push encore")))
				}
			}
		}
	}
	os.Exit(code)
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVar(&tunnel, "tunnel", false, "Create a tunnel to your machine for others to test against")
	runCmd.Flags().BoolVar(&debug, "debug", false, "Compile for debugging (disables some optimizations)")
	runCmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch for changes and live-reload")
	runCmd.Flags().StringVar(&listen, "listen", "localhost", "Address to listen on (for example \"0.0.0.0:4000\")")
	runCmd.Flags().UintVarP(&port, "port", "p", 4000, "Port to listen on")
}
