// Package desktop opens files and folders through the desktop environment of the
// machine the Encore daemon runs on.
//
// It exists for the developer dashboard's "open" and "reveal in file manager"
// actions.
package desktop

import (
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
)

// ErrNoDesktop reports that this machine has no desktop environment.
var ErrNoDesktop = errors.New("this machine has no desktop environment to open files in")

// Supported reports whether Open and Reveal can do anything on this machine.
func Supported() bool {
	return supported()
}

// Open launches path with the program the operating system has registered for it.
//
// The program is detached from the daemon, so it outlives it.
func Open(path string) error {
	if err := check(path); err != nil {
		return err
	}
	return open(path)
}

// Reveal opens the file manager showing the directory holding path, with path
// selected. It accepts both files and directories.
//
// Selecting the item isn't possible on every platform; see the platform files.
func Reveal(path string) error {
	if err := check(path); err != nil {
		return err
	}
	return reveal(path)
}

// check rejects a path that no platform could act on.
func check(path string) error {
	if !supported() {
		return ErrNoDesktop
	}

	if !filepath.IsAbs(path) {
		return errors.Newf("path must be absolute, got %q", path)
	}

	if _, err := os.Stat(path); err != nil {
		return err
	}
	return nil
}
