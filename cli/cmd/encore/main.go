package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/tools/go/packages"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"

	// Register commands
	_ "encr.dev/cli/cmd/encore/app"
	_ "encr.dev/cli/cmd/encore/config"
	_ "encr.dev/cli/cmd/encore/k8s"
	_ "encr.dev/cli/cmd/encore/namespace"
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

func determineWorkspaceRoot(appRoot string) string {
	return cmdutil.WorkspaceRoot(appRoot)
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

func displayError(out *os.File, err []byte) {
	cmdutil.DisplayError(out, err)
}

func fatal(args ...interface{}) {
	cmdutil.Fatal(args...)
}

func fatalf(format string, args ...interface{}) {
	cmdutil.Fatalf(format, args...)
}

func nonZeroPtr[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}
