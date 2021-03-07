package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	daemonpb "encr.dev/proto/encore/daemon"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	tunnel bool
	debug  bool
	watch  bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs your application",
	Run: func(cmd *cobra.Command, args []string) {
		appRoot, _ := determineAppRoot()
		runApp(appRoot, tunnel, watch)
	},
}

// runApp runs the app.
func runApp(appRoot string, tunnel, watch bool) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-interrupt
		cancel()
	}()

	daemon := setupDaemon(ctx)
	stream, err := daemon.Run(ctx, &daemonpb.RunRequest{
		AppRoot: appRoot,
		Tunnel:  tunnel,
		Debug:   debug,
		Watch:   watch,
	})
	if err != nil {
		fatal(err)
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF || status.Code(err) == codes.Canceled {
			return
		} else if err != nil {
			fatal(err)
		}
		switch resp := msg.Msg.(type) {
		case *daemonpb.RunMessage_Started:
			fmt.Fprintf(os.Stderr, "Running on http://localhost:%d\n", resp.Started.Port)
			if debug && resp.Started.Pid > 0 {
				fmt.Fprintf(os.Stderr, "Process ID (for debugging): %d\n", resp.Started.Pid)
			}
			if url := resp.Started.TunnelUrl; url != "" {
				fmt.Fprintf(os.Stderr, "Tunnel active on %s\n", url)
			}

		case *daemonpb.RunMessage_Output:
			if out := resp.Output.Stdout; len(out) > 0 {
				os.Stdout.Write(out)
			}
			if out := resp.Output.Stderr; len(out) > 0 {
				os.Stderr.Write(out)
			}

		case *daemonpb.RunMessage_Exit:
			os.Exit(int(resp.Exit.Code))
		}
	}
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVar(&tunnel, "tunnel", false, "Create a tunnel to your machine for others to test against")
	runCmd.Flags().BoolVar(&debug, "debug", false, "Compile for debugging (disables some optimizations)")
	runCmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch for changes and live-reload")
}
