package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	daemonpb "encr.dev/proto/encore/daemon"
)

var (
	logsEnv  string
	logsJSON bool
)

var logsCmd = &cobra.Command{
	Use:   "logs [--env=prod] [--json]",
	Short: "Streams logs from your application",

	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		appRoot, _ := determineAppRoot()
		streamLogs(appRoot, logsEnv)
	},
}

func streamLogs(appRoot, envName string) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-interrupt
		cancel()
	}()

	daemon := setupDaemon(ctx)
	stream, err := daemon.Logs(ctx, &daemonpb.LogsRequest{
		AppRoot: appRoot,
		EnvName: envName,
	})
	if err != nil {
		fatal("could not stream logs: ", err)
	}

	cw := zerolog.NewConsoleWriter()
	for {
		msg, err := stream.Recv()
		if err == io.EOF || status.Code(err) == codes.Canceled {
			return
		} else if err != nil {
			fatal(err)
		}
		for _, line := range msg.Lines {
			// Pretty-print logs if requested and it looks like a JSON log line
			if !logsJSON && bytes.HasPrefix(line, []byte{'{'}) {
				if _, err := cw.Write(line); err != nil {
					// Fall back to regular stdout in case of error
					os.Stdout.Write(line)
				}
			} else {
				os.Stdout.Write(line)
			}
		}
		if msg.DropNotice {
			fmt.Fprintln(os.Stderr, "--- NOTICE: log lines were not sent due to high volume or slow reader ---")
		}
	}
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().StringVarP(&logsEnv, "env", "e", "", "Environment name to stream logs from (defaults to the production environment)")
	logsCmd.Flags().BoolVar(&logsJSON, "json", false, "Whether to print logs in raw JSON format")
}
