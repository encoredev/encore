package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	daemonpb "encr.dev/proto/encore/daemon"
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

	converter := convertJSONLogs()
	if slices.Contains(args, "-json") {
		converter = convertTestEventOutputOnly(converter)
	}

	daemon := setupDaemon(ctx)
	stream, err := daemon.Test(ctx, &daemonpb.TestRequest{
		AppRoot:    appRoot,
		WorkingDir: testDir,
		Args:       args,
		Environ:    os.Environ(),
	})
	if err != nil {
		fatal(err)
	}
	os.Exit(streamCommandOutput(stream, converter))
}

func init() {
	testCmd.DisableFlagParsing = true
	rootCmd.AddCommand(testCmd)
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
