package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"encr.dev/cli/internal/onboarding"
	daemonpb "encr.dev/proto/encore/daemon"
	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/cobra"
)

var (
	tunnel bool
	debug  bool
	watch  bool
)

var runCmd = &cobra.Command{
	Use:   "run [--debug] [--watch=true]",
	Short: "Runs your application",
	Run: func(cmd *cobra.Command, args []string) {
		appRoot, wd := determineAppRoot()
		runApp(appRoot, wd, tunnel, watch)
	},
}

// runApp runs the app.
func runApp(appRoot, wd string, tunnel, watch bool) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-interrupt
		cancel()
	}()

	daemon := setupDaemon(ctx)
	stream, err := daemon.Run(ctx, &daemonpb.RunRequest{
		AppRoot:    appRoot,
		Tunnel:     tunnel,
		Debug:      debug,
		Watch:      watch,
		WorkingDir: wd,
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
}
