// Command go-encore-wrapper provides a wrapper around the go command
// that invokes the "encore" binary instead for certain commands, like running tests.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

// EncoreBinary is the name of the encore binary to use.
// It's a variable to be able to override it for development purposes.
var EncoreBinary = "encore"

func main() {
	dst := "go"

	// If this is running "go test" inside an Encore app, rewrite it to "encore test".
	if len(os.Args) > 1 && os.Args[1] == "test" && isEncoreApp() {
		dst = EncoreBinary
	}

	cmd := exec.Command(dst, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		os.Exit(1)
	}
}

// isEncoreApp reports whether the working directory is within an Encore app.
func isEncoreApp() bool {
	wd, err := os.Getwd()
	if err != nil {
		return false
	}

	curr := wd
	for i := 0; i < 100; i++ {
		candidate := filepath.Join(curr, "encore.app")
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return true
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	return false
}
