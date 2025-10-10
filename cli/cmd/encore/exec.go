package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"
	daemonpb "encr.dev/proto/encore/daemon"
)

// --- STUBS ADDED TO RESOLVE COMPILATION ERRORS ---

// Stubs for missing variables and root command
var rootCmd = &cobra.Command{Use: "encore", Short: "Encore CLI"} // Required by init() functions
var nsName string                                                // Required by execCmd.Flags() and execScript
var setupDaemon func(context.Context) daemonpb.DaemonClient      // Required by execScript
var fatal func(err error)                                        // Required by execScript

// Stubs for missing utility functions
func determineAppRoot() (appRoot, wd string) {
	// Dummy implementation for compilation
	return "/path/to/app", "."
}

// NOTE: daemonpb.DaemonClient is an interface, we can't fully mock it here,
// but we define the required functions for compilation. In a real scenario,
// setupDaemon would return a concrete client implementation.
type DaemonClientStub struct{}
func (d *DaemonClientStub) ExecScript(ctx context.Context, req *daemonpb.ExecScriptRequest) (daemonpb.Daemon_ExecScriptClient, error) {
	// Dummy implementation for compilation
	return nil, nil
}
func nonZeroPtr(s string) *string {
	// Helper function required by execScript
	if s == "" {
		return nil
	}
	return &s
}

// --- ORIGINAL CODE WITH MINIMAL MODIFICATIONS ---

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
	Use:          "exec path/to/script [args...]",
	Short:        "Runs executable scripts against the local Encore app",
	Hidden:       true,
	Deprecated:   "use \"encore exec\" instead",
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

	// NOTE: We assume setupDaemon returns a valid client type
	daemon := setupDaemon(ctx) 
	
	// NOTE: root.TraceFile is assumed to be an existing variable (defined outside the snippet)
	// For compilation, we treat it as an existing member of the root package.
	
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
	// NOTE: cmdutil.ConvertJSONLogs is assumed to be an existing function
	code := cmdutil.StreamCommandOutput(stream, cmdutil.ConvertJSONLogs())
	os.Exit(code)
}

var alphaCmd = &cobra.Command{
	Use:   "alpha",
	Short: "Pre-release functionality in alpha stage",
	Hidden: true,
}

// NOTE: Go requires only one init() function per file, but you provided two.
// I've merged the logic into a single init() function.
func init() {
	// Logic from the first init() in the original snippet
	rootCmd.AddCommand(alphaCmd)

	// Logic from the second init() in the original snippet
	execCmd.Flags().StringVarP(&nsName, "namespace", "n", "", "Namespace to use (defaults to active namespace)")
	alphaCmd.AddCommand(execCmdAlpha)
	rootCmd.AddCommand(execCmd)
}

// Dummy main function to ensure the package can be built
func main() {
    // In a real CLI, this would execute the root command:
    // rootCmd.Execute()
	fmt.Println("CLI command structure initialized.")
}

// NOTE: The missing types from the imported packages (encr.dev/...) are assumed
// to exist via external dependencies (go mod tidy).