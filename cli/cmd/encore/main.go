package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	daemonpb "encr.dev/proto/encore/daemon"
)

var verbosity int

var rootCmd = &cobra.Command{
	Use:           "encore",
	Short:         "encore is the fastest way of developing backend applications",
	SilenceErrors: true, // We'll handle displaying an error in our main func
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true, // Hide the "completion" command from help (used for generating auto-completions for the shell)
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := zerolog.InfoLevel
		if verbosity == 1 {
			level = zerolog.DebugLevel
		} else if verbosity >= 2 {
			level = zerolog.TraceLevel
		}
		log.Logger = log.Logger.Level(level)
	},
}

func main() {
	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "verbose output")
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	if err := rootCmd.Execute(); err != nil {
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
	dir, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	rel := "."
	for {
		path := filepath.Join(dir, "encore.app")
		fi, err := os.Stat(path)
		if os.IsNotExist(err) {
			dir2 := filepath.Dir(dir)
			if dir2 == dir {
				fatal("no encore.app found in directory (or any of the parent directories).")
			}
			rel = filepath.Join(filepath.Base(dir), rel)
			dir = dir2
			continue
		} else if err != nil {
			fatal(err)
		} else if fi.IsDir() {
			fatal("encore.app is a directory, not a file")
		} else {
			return dir, rel
		}
	}
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

// streamCommandOutput streams the output from the given command stream,
// and reports the command's exit code.
func streamCommandOutput(stream commandOutputStream) int {
	for {
		msg, err := stream.Recv()
		if err != nil {
			st := status.Convert(err)
			switch {
			case st.Code() == codes.FailedPrecondition:
				fmt.Fprintln(os.Stderr, st.Message())
				return 1
			case err == io.EOF || st.Code() == codes.Canceled:
				return 0
			default:
				log.Fatal().Err(err).Msg("connection failure")
			}
		}

		switch m := msg.Msg.(type) {
		case *daemonpb.CommandMessage_Output:
			if m.Output.Stdout != nil {
				os.Stdout.Write(m.Output.Stdout)
			}
			if m.Output.Stderr != nil {
				os.Stderr.Write(m.Output.Stderr)
			}
		case *daemonpb.CommandMessage_Exit:
			return int(m.Exit.Code)
		}
	}
}

func fatal(args ...interface{}) {
	// Prettify gRPC errors
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			if s, ok := status.FromError(err); ok {
				args[i] = s.Message()
			}
		}
	}

	red := color.New(color.FgRed)
	red.Fprint(os.Stderr, "error: ")
	red.Fprintln(os.Stderr, args...)
	os.Exit(1)
}

func fatalf(format string, args ...interface{}) {
	// Prettify gRPC errors
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			if s, ok := status.FromError(err); ok {
				args[i] = s.Message()
			}
		}
	}

	fatal(fmt.Sprintf(format, args...))
}
