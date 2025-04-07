package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"
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
var execCmdAlpha = &cobra.Command{
	Use:        "exec path/to/script [args...]",
	Short:      "Runs executable scripts against the local Encore app",
	Hidden:     true,
	Deprecated: "use \"encore exec\" instead",
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

	daemon := setupDaemon(ctx)
	stream, err := daemon.ExecScript(ctx, &daemonpb.ExecScriptRequest{
		AppRoot:    appRoot,
		WorkingDir: relWD,
		ScriptArgs: args,
		Environ:    os.Environ(),
		TraceFile:  root.TraceFile,
		Namespace:  nonZeroPtr(nsName),
	})
	if err != nil {
		fatal(err)
	}

	cmdutil.ClearTerminalExceptFirstNLines(1)
	code := cmdutil.StreamCommandOutput(stream, cmdutil.ConvertJSONLogs())
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
	execCmd.Flags().StringVarP(&nsName, "namespace", "n", "", "Namespace to use (defaults to active namespace)")
	alphaCmd.AddCommand(execCmdAlpha)
	rootCmd.AddCommand(execCmd)
}
