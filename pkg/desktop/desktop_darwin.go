//go:build darwin

package desktop

import (
	"encr.dev/internal/env"
)

// open hands the path to the same opener the `open` command uses in a terminal.
// "--" keeps a path starting with "-" from being read as a flag.
func open(path string) error {
	return runCmd("open", "--", path)
}

// reveal uses `open -R`, which opens the enclosing folder in Finder with the item
// selected.
func reveal(path string) error {
	return runCmd("open", "-R", "--", path)
}

// supported reports false over SSH: `open` needs a logged-in window session to
// hand the file to, and an SSH session has none.
func supported() bool {
	return !env.IsSSH()
}
