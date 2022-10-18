// Package env answers where Encore tools and resources are located.
package env

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// These can be overwritten using
// `go build -ldflags "-X encr.dev/cli/internal/env.alternativeEncoreRuntimePath=$HOME/src/github.com/encoredev/encore/runtime"`.
var (
	alternativeEncoreRuntimePath = ""
	alternativeEncoreGoPath      = ""
)

// EncoreRuntimePath reports the path to the Encore runtime.
// It can be overridden by setting ENCORE_RUNTIME_PATH.
func EncoreRuntimePath() string {
	p := encoreRuntimePath()
	if p == "" {
		log.Fatal().Msg("could not determine Encore install root. " +
			"You can specify the path to the Encore runtime manually by setting the ENCORE_RUNTIME_PATH environment variable.")
	}
	return p
}

// EncoreGoRoot reports the path to the Encore Go root.
// It can be overridden by setting ENCORE_GOROOT.
func EncoreGoRoot() string {
	p := encoreGoRoot()
	if p == "" {
		log.Fatal().Msg("could not determine Encore install root. " +
			"You can specify the path to the Encore GOROOT manually by setting the ENCORE_GOROOT environment variable.")
	}
	return p
}

func encoreRuntimePath() string {
	if p := os.Getenv("ENCORE_RUNTIME_PATH"); p != "" {
		return p
	} else if //goland:noinspection GoBoolExpressions
	alternativeEncoreRuntimePath != "" {
		return alternativeEncoreRuntimePath
	} else if root, ok := determineRoot(); ok {
		return filepath.Join(root, "runtime")
	}
	return ""
}

func encoreGoRoot() string {
	if p := os.Getenv("ENCORE_GOROOT"); p != "" {
		return p
	} else if //goland:noinspection GoBoolExpressions
	alternativeEncoreGoPath != "" {
		return alternativeEncoreGoPath
	} else if root, ok := determineRoot(); ok {
		return filepath.Join(root, "encore-go")
	}
	return ""
}

// List reports Encore environment variables, in the same format as os.Environ().
func List() []string {
	return []string{
		"ENCORE_GOROOT=" + encoreGoRoot(),
		"ENCORE_RUNTIME_PATH=" + encoreRuntimePath(),
	}
}

// determineRoot determines encore root by checking the location relative
// to the executable, to enable relocatable installs.
func determineRoot() (root string, ok bool) {
	exe, err := os.Executable()
	if err == nil {
		// Homebrew uses a lot of symlinks, so we need to get back to the actual location
		// to be able to use the heuristic below.
		if sym, err := filepath.EvalSymlinks(exe); err == nil {
			exe = sym
		}

		root := filepath.Dir(filepath.Dir(exe))
		// Heuristic: check if "encore-go" and "runtime" dirs exist in this location.
		_, err1 := os.Stat(filepath.Join(root, "encore-go"))
		_, err2 := os.Stat(filepath.Join(root, "runtime"))
		if err1 == nil && err2 == nil {
			return root, true
		}
	}
	return "", false
}
