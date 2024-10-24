package cmdutil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/logrusorgru/aurora/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"encr.dev/pkg/ansi"
	"encr.dev/proto/encore/daemon"
)

// CommandOutputStream is the interface for gRPC streams that
// stream the output of a command.
type CommandOutputStream interface {
	Recv() (*daemon.CommandMessage, error)
}

type OutputConverter func(line []byte) []byte

// StreamCommandOutput streams the output from the given command stream,
// and reports the command's exit code.
// If convertJSON is true, lines that look like JSON are fed through
// zerolog's console writer.
func StreamCommandOutput(stream CommandOutputStream, converter OutputConverter) int {
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

				for {
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
					if err := scanner.Err(); err != nil {
						// The scanner failed, likely due to a too-long line. Log an error
						// and create a new scanner since the old one is in an unrecoverable state.
						fmt.Fprintln(os.Stderr, "failed to read output:", err)
						scanner = bufio.NewScanner(read)
						continue
					} else {
						break
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
		case *daemon.CommandMessage_Output:
			if m.Output.Stdout != nil {
				_, _ = outWrite.Write(m.Output.Stdout)
			}
			if m.Output.Stderr != nil {
				_, _ = errWrite.Write(m.Output.Stderr)
			}
		case *daemon.CommandMessage_Errors:
			DisplayError(os.Stderr, m.Errors.Errinsrc)

		case *daemon.CommandMessage_Exit:
			return int(m.Exit.Code)
		}
	}
}

type ConvertLogOptions struct {
	Color bool
}

type ConvertLogOption func(*ConvertLogOptions)

func Colorize(enable bool) ConvertLogOption {
	return func(clo *ConvertLogOptions) {
		clo.Color = enable
	}
}

func ConvertJSONLogs(opts ...ConvertLogOption) OutputConverter {
	// Default to colorized output.
	options := ConvertLogOptions{Color: true}

	for _, opt := range opts {
		opt(&options)
	}

	var logMutex sync.Mutex
	logLineBuffer := bytes.NewBuffer(make([]byte, 0, 1024))
	cout := zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = logLineBuffer
		w.FieldsExclude = []string{"stack"}
		w.FormatExtra = func(vals map[string]any, buf *bytes.Buffer) error {
			if stack, ok := vals["stack"]; ok {
				return FormatStack(stack, buf)
			}
			return nil
		}
	})
	if !options.Color {
		cout.NoColor = true
	}

	return func(line []byte) []byte {
		// If this isn't a JSON log line, just return it as-is
		if len(line) == 0 || line[0] != '{' {
			return line
		}

		// Otherwise grab the converter buffer and reset it
		logMutex.Lock()
		defer logMutex.Unlock()
		logLineBuffer.Reset()

		// Then convert the JSON log line to pretty formatted text
		_, err := cout.Write(line)
		if err != nil {
			return line
		}
		out := make([]byte, len(logLineBuffer.Bytes()))
		copy(out, logLineBuffer.Bytes())
		return out
	}
}

func FormatStack(val any, buf *bytes.Buffer) error {
	var frames []struct {
		File string
		Line int
		Func string
	}

	if jsonRepr, err := json.Marshal(val); err != nil {
		return err
	} else if err := json.Unmarshal(jsonRepr, &frames); err != nil {
		return err
	}
	for _, f := range frames {
		fmt.Fprintf(buf, "\n    %s\n        %s",
			f.Func,
			aurora.Gray(12, fmt.Sprintf("%s:%d", f.File, f.Line)))
	}
	return nil
}

func ClearTerminalExceptFirstNLines(n int) {
	// Clear the screen except for the first line.
	if _, height, err := terminal.GetSize(int(os.Stdout.Fd())); err == nil {
		count := height - (1 + n)
		if count > 0 {
			_, _ = os.Stdout.Write(bytes.Repeat([]byte{'\n'}, count))
		}
		_, _ = fmt.Fprint(os.Stdout, ansi.SetCursorPosition(2, 1)+ansi.ClearScreen(ansi.CursorToBottom))
	}
}
