package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	daemonpb "encr.dev/proto/encore/daemon"
)

var checkTimeout time.Duration

func init() {
	checkCmd := &cobra.Command{
		Use:   "check [spec]",
		Short: "Runs a list of HTTP requests against a fresh app instance",
		Long: `Compiles the application, starts up required infrastructure resources,
and waits for the application to start. Then runs each command from the spec
against the app and writes the results to stdout. Shuts down the application afterwards.

The spec is taken from the positional argument (if given), or from stdin (if
piped/redirected), or empty otherwise. An empty spec just verifies that the
app successsfully starts up. Pass a file with shell redirection,
e.g. 'encore check < spec.txt'.

Spec format: one command per line. Multiple commands on a single line can be
separated by ';'. Blank lines and lines starting with '#' are ignored.

  curl <path> [args...]    # path must start with '/'.
                           # remaining tokens are passed verbatim to curl.

Examples:
  encore check
  encore check 'curl /ping'
  encore check 'curl /ping; curl /hello/world -i'
  encore check < spec.txt

Output: each command result is printed to stdout framed by '=== [i/N] ... ===' lines.
Daemon and app log output goes to stderr. Exit code is 0 when every command
exited 0; if the app failed to start or any command failed, the exit code is 1.
`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var (
				spec   io.Reader
				source string
			)
			switch {
			case len(args) == 1:
				spec = strings.NewReader(args[0])
				source = "<spec>"
			case !term.IsTerminal(int(os.Stdin.Fd())):
				spec = os.Stdin
				source = "<stdin>"
			default:
				spec = strings.NewReader("")
				source = "<spec>"
			}
			runCheck(spec, source)
		},
	}
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().DurationVar(&checkTimeout, "timeout", 60*time.Second, "How long to wait for /__encore/healthz before giving up")
}

func runCheck(spec io.Reader, source string) {
	cmds, err := parseSpec(spec, source)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	appRoot, wd := determineAppRoot()
	daemon := setupDaemon(ctx)

	stream, err := daemon.RunSpec(ctx, &daemonpb.RunSpecRequest{
		AppRoot:             appRoot,
		WorkingDir:          wd,
		Environ:             os.Environ(),
		Commands:            cmds,
		ReadyTimeoutSeconds: int32(checkTimeout.Seconds()),
	})
	if err != nil {
		fatal(err)
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			fmt.Fprintln(os.Stderr, "check: stream closed unexpectedly without a completion message")
			os.Exit(1)
		} else if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				os.Exit(130)
			}
			fmt.Fprintf(os.Stderr, "check: %v\n", err)
			os.Exit(1)
		}
		switch m := msg.Msg.(type) {
		case *daemonpb.RunSpecMessage_Output:
			if len(m.Output.Stderr) > 0 {
				_, _ = os.Stderr.Write(m.Output.Stderr)
			}
			if len(m.Output.Stdout) > 0 {
				// Daemon log noise routes to *our* stderr — stdout is for results only.
				_, _ = os.Stderr.Write(m.Output.Stdout)
			}
		case *daemonpb.RunSpecMessage_Result:
			renderResult(m.Result)
		case *daemonpb.RunSpecMessage_Complete:
			renderComplete(m.Complete)
			if m.Complete.Error != "" || m.Complete.Succeeded != m.Complete.Total {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
}

func renderResult(r *daemonpb.SpecCommandResult) {
	fmt.Fprintf(os.Stdout, "=== [%d/%d] %s ===\n", r.Index, r.Total, r.Display)
	if len(r.Stdout) > 0 {
		_, _ = os.Stdout.Write(r.Stdout)
		if r.Stdout[len(r.Stdout)-1] != '\n' {
			fmt.Fprintln(os.Stdout)
		}
	}
	if len(r.Stderr) > 0 {
		fmt.Fprintln(os.Stdout, "--- stderr ---")
		_, _ = os.Stdout.Write(r.Stderr)
		if r.Stderr[len(r.Stderr)-1] != '\n' {
			fmt.Fprintln(os.Stdout)
		}
	}
	fmt.Fprintf(os.Stdout, "--- exit: %d ---\n", r.ExitCode)
}

func renderComplete(c *daemonpb.SpecComplete) {
	if c.Error != "" {
		fmt.Fprintf(os.Stderr, "check: %s\n", c.Error)
		return
	}
	fmt.Fprintf(os.Stderr, "check: %d/%d commands succeeded\n", c.Succeeded, c.Total)
}

// parseSpec parses a spec from r. source is used in error messages.
// Lines may contain multiple commands separated by ';'. Blank lines and
// lines starting with '#' are ignored. Currently only `curl <path> [args...]`
// is supported.
func parseSpec(r io.Reader, source string) ([]*daemonpb.SpecCommand, error) {
	var cmds []*daemonpb.SpecCommand
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // allow long lines (e.g. JSON bodies)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		segments, err := tokenize(raw)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %v", source, lineNo, err)
		}
		for _, tokens := range segments {
			if len(tokens) == 0 {
				continue
			}
			switch tokens[0] {
			case "curl":
				if len(tokens) < 2 {
					return nil, fmt.Errorf("%s:%d: curl requires a path argument", source, lineNo)
				}
				pathArg := tokens[1]
				if !strings.HasPrefix(pathArg, "/") {
					return nil, fmt.Errorf("%s:%d: curl path %q must be relative (start with '/')", source, lineNo, pathArg)
				}
				cmds = append(cmds, &daemonpb.SpecCommand{
					Cmd: &daemonpb.SpecCommand_Curl{Curl: &daemonpb.CurlCommand{
						Path: pathArg,
						Args: tokens[2:],
					}},
				})
			default:
				return nil, fmt.Errorf("%s:%d: unsupported command %q (only 'curl' is supported)", source, lineNo, tokens[0])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("check: read %s: %v", source, err)
	}
	return cmds, nil
}

// tokenize splits a spec line into one or more command segments, each a list
// of shell-style tokens. A top-level ';' (outside any quotes) ends a segment.
// Handles single quotes (literal), double quotes (with backslash escapes for
// \" and \\), and backslash escapes outside quotes. This is a small subset of
// POSIX shell tokenization sufficient for curl-style argument lists.
func tokenize(line string) ([][]string, error) {
	var (
		segments [][]string
		tokens   []string
		cur      strings.Builder
		inTok    bool
	)
	const (
		none = iota
		single
		double
	)
	quote := none
	flushTok := func() {
		if inTok {
			tokens = append(tokens, cur.String())
			cur.Reset()
			inTok = false
		}
	}
	flushSeg := func() {
		flushTok()
		if len(tokens) > 0 {
			segments = append(segments, tokens)
			tokens = nil
		}
	}
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch quote {
		case single:
			if r == '\'' {
				quote = none
				continue
			}
			cur.WriteRune(r)
			inTok = true
		case double:
			if r == '\\' && i+1 < len(runes) {
				next := runes[i+1]
				if next == '"' || next == '\\' {
					cur.WriteRune(next)
					inTok = true
					i++
					continue
				}
			}
			if r == '"' {
				quote = none
				continue
			}
			cur.WriteRune(r)
			inTok = true
		default:
			switch r {
			case ' ', '\t':
				flushTok()
			case ';':
				flushSeg()
			case '\'':
				quote = single
				inTok = true
			case '"':
				quote = double
				inTok = true
			case '\\':
				if i+1 < len(runes) {
					i++
					cur.WriteRune(runes[i])
					inTok = true
				}
			default:
				cur.WriteRune(r)
				inTok = true
			}
		}
	}
	if quote != none {
		return nil, errors.New("unterminated quote")
	}
	flushSeg()
	return segments, nil
}
