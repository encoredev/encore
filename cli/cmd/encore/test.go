package main

import (
	"context"
	"os"
	"os/signal"

	daemonpb "encr.dev/proto/encore/daemon"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test [go test flags]",
	Short: "Tests your application",
	Long:  "Takes all the same flags as `go test`.",

	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		// Support --help but otherwise let all args be passed on to "go test"
		for _, arg := range args {
			if arg == "-h" || arg == "--help" {
				cmd.Help()
				return
			}
		}

		appRoot, relPath := determineAppRoot()
		runTests(appRoot, relPath, args)
	},
}

func runTests(appRoot, testDir string, args []string) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-interrupt
		cancel()
	}()

	daemon := setupDaemon(ctx)
	stream, err := daemon.Test(ctx, &daemonpb.TestRequest{
		AppRoot:    appRoot,
		WorkingDir: testDir,
		Args:       args,
	})
	if err != nil {
		fatal(err)
	}
	os.Exit(streamCommandOutput(stream))
}

func init() {
	testCmd.DisableFlagParsing = true
	rootCmd.AddCommand(testCmd)
}
