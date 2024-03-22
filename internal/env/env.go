// Package env answers where Encore tools and resources are located.
package env

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"encr.dev/pkg/option"
)

// These can be overwritten using
// `go build -ldflags "-X encr.dev/cli/internal/env.alternativeEncoreRuntimesPath=$HOME/src/github.com/encoredev/encore/runtimes"`.
var (
	alternativeEncoreRuntimesPath = ""
	alternativeEncoreGoPath       = ""
)

// EncoreRuntimesPath reports the path to the Encore runtime.
// It can be overridden by setting ENCORE_RUNTIMES_PATH.
func EncoreRuntimesPath() string {
	p := encoreRuntimesPath()
	if p == "" {
		log.Fatal().Msg("could not determine Encore install root. " +
			"You can specify the path to the Encore runtimes manually by setting the ENCORE_RUNTIMES_PATH environment variable.")
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

// EncoreBin reports the path to the directory containing the Encore installation's binaries.
func EncoreBin() option.Option[string] {
	if root, ok := determineRoot(); ok {
		return option.Some(filepath.Join(root, "bin"))
	}
	return option.None[string]()
}

// OptEncoreGoRoot reports the path to the Encore Go root.
// It can be overridden by setting ENCORE_GOROOT.
// If the goroot can't be found, it reports None.
func OptEncoreGoRoot() option.Option[string] {
	return option.AsOptional(encoreGoRoot())
}

func encoreRuntimesPath() string {
	if p := os.Getenv("ENCORE_RUNTIMES_PATH"); p != "" {
		return p
	} else if //goland:noinspection GoBoolExpressions
	alternativeEncoreRuntimesPath != "" {
		return alternativeEncoreRuntimesPath
	} else if root, ok := determineRoot(); ok {
		return filepath.Join(root, "runtimes")
	}
	return ""
}

// EncoreRuntimeLib reports the path to the Encore runtime library for
// node.js. It can be overridden by setting ENCORE_RUNTIME_LIB.
func EncoreRuntimeLib() string {
	if p := os.Getenv("ENCORE_RUNTIME_LIB"); p != "" {
		return p
	} else if rt := encoreRuntimesPath(); rt != "" {
		return filepath.Join(rt, "js", "encore-runtime.node")
	}
	return ""
}

// EncoreDaemonLogPath reports the path to the Encore daemon log file.
// It can be overridden by setting ENCORE_DAEMON_LOG_PATH.
func EncoreDaemonLogPath() string {
	if p := os.Getenv("ENCORE_DAEMON_LOG_PATH"); p != "" {
		return p
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		log.Fatal().Err(err).Msg("unable to determine user cache directory")
	}
	return filepath.Join(cache, "encore", "daemon.log")
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
		"ENCORE_RUNTIMES_PATH=" + encoreRuntimesPath(),
		"ENCORE_RUNTIME_LIB=" + EncoreRuntimeLib(),
		"ENCORE_DAEMON_LOG_PATH=" + EncoreDaemonLogPath(),
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
		_, err2 := os.Stat(filepath.Join(root, "runtimes", "go"))
		if err1 == nil && err2 == nil {
			return root, true
		}
	}
	return "", false
}

// IsSSH reports whether the current session is an SSH session.
func IsSSH() bool {
	if os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" {
		return true
	}
	return false
}
