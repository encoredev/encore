package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/spf13/cobra"

	daemonpb "encr.dev/proto/encore/daemon"
)

var execCmd = &cobra.Command{
	Use:   "exec path/to/script [args...]",
	Short: "Runs executable scripts against the local Encore app",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			args = []string{"."} // current directory
		}
		appRoot, wd := determineAppRoot()
		execScript(appRoot, wd, args)
	},
}

func execScript(appRoot, relWD string, args []string) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-interrupt
		cancel()
	}()

	scriptPath := filepath.Join(relWD, args[0])
	scriptArgs := args[1:]

	daemon := setupDaemon(ctx)
	stream, err := daemon.ExecScript(ctx, &daemonpb.ExecScriptRequest{
		AppRoot:       appRoot,
		WorkingDir:    relWD,
		ScriptRelPath: scriptPath,
		ScriptArgs:    scriptArgs,
		Environ:       os.Environ(),
	})
	if err != nil {
		fatal(err)
	}

	clearTerminalExceptFirstLine()
	code := streamCommandOutput(stream, convertJSONLogs())
	os.Exit(code)
}

var alphaCmd = &cobra.Command{
	Use:    "alpha",
	Short:  "Pre-release functionality in alpha stage",
	Hidden: true,
}

func init() {
	rootCmd.AddCommand(alphaCmd)
}

func init() {
	alphaCmd.AddCommand(execCmd)
}
