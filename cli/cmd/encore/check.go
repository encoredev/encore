package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	daemonpb "encr.dev/proto/encore/daemon"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Checks your application for errors",
	Run: func(cmd *cobra.Command, args []string) {
		appRoot, relPath := determineAppRoot()
		runChecks(appRoot, relPath)
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

func runChecks(appRoot, relPath string) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-interrupt
		cancel()
	}()

	daemon := setupDaemon(ctx)
	stream, err := daemon.Check(ctx, &daemonpb.CheckRequest{AppRoot: appRoot, WorkingDir: relPath})
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal: ", err)
		os.Exit(1)
	}
	os.Exit(streamCommandOutput(stream))
}
