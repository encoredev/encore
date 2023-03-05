package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/tools/go/packages"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"
	daemonpb "encr.dev/proto/encore/daemon"
	// Register commands
	_ "encr.dev/cli/cmd/encore/app"
	_ "encr.dev/cli/cmd/encore/secrets"
)

// for backwards compatibility, for now
var rootCmd = root.Cmd

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	if err := root.Cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// determineAppRoot determines the app root by looking for the "encore.app" file,
// initially in the current directory and then recursively in parent directories
// up to the filesystem root.
// It reports the absolute path to the app root, and the
// relative path from the app root to the working directory.
// On errors it prints an error message and exits.
func determineAppRoot() (appRoot, relPath string) {
	return cmdutil.AppRoot()
}

func resolvePackages(dir string, patterns ...string) ([]string, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		paths = append(paths, pkg.PkgPath)
	}
	return paths, nil
}

// commandOutputStream is the interface for gRPC streams that
// stream the output of a command.
type commandOutputStream interface {
	Recv() (*daemonpb.CommandMessage, error)
}

type outputConverter func(line []byte) []byte

// streamCommandOutput streams the output from the given command stream,
// and reports the command's exit code.
// If convertJSON is true, lines that look like JSON are fed through
// zerolog's console writer.
func streamCommandOutput(stream commandOutputStream, converter outputConverter) int {
	var outWrite io.Writer = os.Stdout
	var errWrite io.Writer = os.Stderr

	var writesDone sync.WaitGroup
	defer writesDone.Wait()

	if converter != nil {
		// Create a pipe that we read from line-by-line so we can detect JSON lines.
		outRead, outw := io.Pipe()
		errRead, errw := io.Pipe()
		outWrite = outw
		errWrite = errw
		defer func() { _ = outw.Close() }()
		defer func() { _ = errw.Close() }()

		for i, read := range []io.Reader{outRead, errRead} {
			read := read
			stdout := i == 0
			writesDone.Add(1)
			go func() {
				defer writesDone.Done()
				scanner := bufio.NewScanner(read)
				for scanner.Scan() {
					line := append(scanner.Bytes(), '\n')
					line = converter(line)
					if stdout {
						_, _ = os.Stdout.Write(line)
					} else {
						_, _ = os.Stderr.Write(line)
					}
				}
			}()
		}
	}

	for {
		msg, err := stream.Recv()
		if err != nil {
			st := status.Convert(err)
			switch {
			case st.Code() == codes.FailedPrecondition:
				_, _ = fmt.Fprintln(os.Stderr, st.Message())
				return 1
			case err == io.EOF || st.Code() == codes.Canceled || strings.HasSuffix(err.Error(), "error reading from server: EOF"):
				return 0
			default:
				log.Fatal().Err(err).Msg("connection failure")
			}
		}

		switch m := msg.Msg.(type) {
		case *daemonpb.CommandMessage_Output:
			if m.Output.Stdout != nil {
				_, _ = outWrite.Write(m.Output.Stdout)
			}
			if m.Output.Stderr != nil {
				_, _ = errWrite.Write(m.Output.Stderr)
			}
		case *daemonpb.CommandMessage_Errors:
			displayError(os.Stderr, m.Errors.Errinsrc)

		case *daemonpb.CommandMessage_Exit:
			return int(m.Exit.Code)
		}
	}
}

func convertJSONLogs() outputConverter {
	var logMutex sync.Mutex
	logLineBuffer := bytes.NewBuffer(make([]byte, 0, 1024))
	cout := zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = logLineBuffer
	})

	return func(line []byte) []byte {
		// If this isn't a JSON log line, just return it as-is
		if len(line) == 0 || line[0] != '{' {
			return line
		}

		// Otherwise grab the the converter buffer and reset it
		logMutex.Lock()
		defer logMutex.Unlock()
		logLineBuffer.Reset()

		// Then convert the JSON log line to pretty formatted text
		_, _ = cout.Write(line)
		out := make([]byte, len(logLineBuffer.Bytes()))
		copy(out, logLineBuffer.Bytes())
		return out
	}
}

func displayError(out *os.File, err []byte) {
	cmdutil.DisplayError(out, err)
}

func fatal(args ...interface{}) {
	cmdutil.Fatal(args...)
}

func fatalf(format string, args ...interface{}) {
	cmdutil.Fatalf(format, args...)
}
