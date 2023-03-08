package testutil

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	"encr.dev/internal/env"
)

var (
	EncoreRepoDir = repoDir()
	RuntimeDir    = filepath.Join(EncoreRepoDir, "runtime")
)

func init() {
	// If we're not in the Encore repo, use the runtime path from the environment.
	if _, err := os.Stat(filepath.Join(EncoreRepoDir, "go.mod")); err != nil {
		RuntimeDir = env.EncoreRuntimePath()
	}
}

func repoDir() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic("unable to get repo directory")
	}
	return string(bytes.TrimSpace(out))
}
