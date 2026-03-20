package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"
	"encr.dev/pkg/appfile"
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

	// For TypeScript apps, use ExecSpec to get the command spec and run it
	// locally. This allows interactive commands (stdin) to work properly.
	lang, err := appfile.AppLang(appRoot)
	if err != nil {
		fatal(err)
	}
	if lang == appfile.LangTS {
		tempDir, err := os.MkdirTemp("", "encore-exec")
		if err != nil {
			fatal(err)
		}
		defer func() { _ = os.RemoveAll(tempDir) }()

		stream, err := daemon.ExecSpec(ctx, &daemonpb.ExecSpecRequest{
			AppRoot:    appRoot,
			WorkingDir: relWD,
			ScriptArgs: args,
			Environ:    os.Environ(),
			Namespace:  nonZeroPtr(nsName),
			TempDir:    tempDir,
		})
		if err != nil {
			fatal(err)
		}

		cmdutil.ClearTerminalExceptFirstNLines(1)

		// Read progress messages until we get the spec.
		var spec *daemonpb.ExecSpecResponse
		for {
			msg, err := stream.Recv()
			if err != nil {
				fatal(err)
			}
			switch m := msg.Msg.(type) {
			case *daemonpb.ExecSpecMessage_Output:
				if len(m.Output.Stdout) > 0 {
					os.Stdout.Write(m.Output.Stdout)
				}
				if len(m.Output.Stderr) > 0 {
					os.Stderr.Write(m.Output.Stderr)
				}
			case *daemonpb.ExecSpecMessage_Spec:
				spec = m.Spec
			}
			if spec != nil {
				break
			}
		}

		cmd := exec.Command(spec.Command, spec.Args...)
		cmd.Env = spec.Environ
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				os.Exit(exitErr.ExitCode())
			}
			fatal(err)
		}
		return
	}

	// For Go apps, use the streaming ExecScript RPC.
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
