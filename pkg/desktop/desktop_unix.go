//go:build !darwin && !windows

package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
)

// open runs xdg-open, the desktop-agnostic opener.
func open(path string) error {
	return runCmd("xdg-open", path)
}

// reveal opens the directory holding path instead of the enclosing one, since it
// is non-trivial to select an item on unix.
func reveal(path string) error {
	dir := path
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		dir = filepath.Dir(path)
	}
	return runCmd("xdg-open", dir)
}

// supported reports whether there is a desktop session for xdg-open to hand a
// path to, and an xdg-open to do it with.
func supported() bool {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return false
	}
	_, err := exec.LookPath("xdg-open")
	return err == nil
}
