package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	daemonpb "encr.dev/proto/encore/daemon"
)

var testCmd = &cobra.Command{
	Use:   "test [go test flags]",
	Short: "Tests your application",
	Long:  "Takes all the same flags as `go test`.",

	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		var (
			traceFile    string
			codegenDebug bool
		)
		// Support specific args but otherwise let all args be passed on to "go test"
		for i := 0; i < len(args); i++ {
			arg := args[i]
			if arg == "-h" || arg == "--help" {
				_ = cmd.Help()
				return
			} else if arg == "--trace" || strings.HasPrefix(arg, "--trace=") {
				// Drop this argument always.
				args = slices.Delete(args, i, i+1)
				i--

				// We either have '--trace=file' or '--trace file'.
				// Handle both.
				if _, value, ok := strings.Cut(arg, "="); ok {
					traceFile = value
				} else {
					// Make sure there is a next argument.
					if i < len(args) {
						traceFile = args[i]
						args = slices.Delete(args, i, i+1)
						i--
					}
				}
			} else if arg == "--codegen-debug" {
				codegenDebug = true
				args = slices.Delete(args, i, i+1)
				i--
			}
		}

		appRoot, relPath := determineAppRoot()
		runTests(appRoot, relPath, args, traceFile, codegenDebug)
	},
}

func runTests(appRoot, testDir string, args []string, traceFile string, codegenDebug bool) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-interrupt
		cancel()
	}()

	converter := convertJSONLogs()
	if slices.Contains(args, "-json") {
		converter = convertTestEventOutputOnly(converter)
	}

	daemon := setupDaemon(ctx)

	stream, err := daemon.Test(ctx, &daemonpb.TestRequest{
		AppRoot:      appRoot,
		WorkingDir:   testDir,
		Args:         args,
		Environ:      os.Environ(),
		TraceFile:    nonZeroPtr(traceFile),
		CodegenDebug: codegenDebug,
	})
	if err != nil {
		fatal(err)
	}
	os.Exit(streamCommandOutput(stream, converter))
}

func init() {
	testCmd.DisableFlagParsing = true
	rootCmd.AddCommand(testCmd)

	// Even though we've disabled flag parsing, we still need to define the flags
	// so that the help text is correct.
	testCmd.Flags().Bool("codegen-debug", false, "Dump generated code (for debugging Encore's code generation)")
	testCmd.Flags().String("trace", "", "Specifies a trace file to write trace information about the parse and compilation process to.")

}

func convertTestEventOutputOnly(converter outputConverter) outputConverter {
	return func(line []byte) []byte {
		// If this isn't a JSON log line, just return it as-is
		if len(line) == 0 || line[0] != '{' {
			return line
		}

		testEvent := &testJSONEvent{}
		if err := json.Unmarshal(line, testEvent); err == nil && testEvent.Action == "output" {
			if testEvent.Output != nil && (*(testEvent.Output))[0] == '{' {
				convertedLogs := textBytes(converter(*testEvent.Output))
				testEvent.Output = &convertedLogs

				newLine, err := json.Marshal(testEvent)
				if err == nil {
					return append(newLine, '\n')
				}
			}
		}

		return line
	}
}

// testJSONEvent and textBytes taken from the Go source code
type testJSONEvent struct {
	Time    *time.Time `json:",omitempty"`
	Action  string
	Package string     `json:",omitempty"`
	Test    string     `json:",omitempty"`
	Elapsed *float64   `json:",omitempty"`
	Output  *textBytes `json:",omitempty"`
}

// textBytes is a hack to get JSON to emit a []byte as a string
// without actually copying it to a string.
// It implements encoding.TextMarshaler, which returns its text form as a []byte,
// and then json encodes that text form as a string (which was our goal).
type textBytes []byte

func (b *textBytes) MarshalText() ([]byte, error) { return *b, nil }
func (b *textBytes) UnmarshalText(in []byte) error {
	*b = in
	return nil
}
