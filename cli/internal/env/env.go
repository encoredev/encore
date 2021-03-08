// Package env answers where Encore tools and resources are located.
package env

import (
	"fmt"
	"os"
	"path/filepath"
)

// EncoreRuntimePath reports the path to the Encore runtime.
// It can be overridden by setting ENCORE_RUNTIME_PATH.
func EncoreRuntimePath() string {
	if p := os.Getenv("ENCORE_RUNTIME_PATH"); p != "" {
		return p
	}
	root, ok := determineRoot()
	if !ok {
		fmt.Fprintln(os.Stderr, "fatal: could not determine Encore install root.\n"+
			"You can specify the path to the Encore runtime manually by setting the ENCORE_RUNTIME_PATH environment variable.")
		os.Exit(1)
	}
	return filepath.Join(root, "runtime")
}

// EncoreGoRoot reports the path to the Encore Go root.
// It can be overridden by setting ENCORE_GOROOT.
func EncoreGoRoot() string {
	if p := os.Getenv("ENCORE_GOROOT"); p != "" {
		return p
	}
	root, ok := determineRoot()
	if !ok {
		fmt.Fprintln(os.Stderr, "fatal: could not determine Encore install root.\n"+
			"You can specify the path to the Encore GOROOT manually by setting the ENCORE_GOROOT environment variable.")
		os.Exit(1)
	}
	return filepath.Join(root, "encore-go")
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
