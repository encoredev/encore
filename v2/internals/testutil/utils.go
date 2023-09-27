package testutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"encr.dev/internal/env"
)

var (
	EncoreRepoDir = repoDir()
	RuntimeDir    = filepath.Join(EncoreRepoDir, "runtimes")
)

// EnvRepoDirOverride is the name of the environment variable to override
// the resolution of the path to this repository. It exists because
// we have some tests that run in a temporary directory (such as when using
// testscript in process execution mode, like compiler/build/build_test.go),
// in which case the normal approach of invoking `git rev-parse --show-toplevel`
// doesn't work.
const EnvRepoDirOverride = "ENCORE_REPO_DIR"

func init() {
	// If we're not in the Encore repo, use the runtime path from the environment.
	if _, err := os.Stat(filepath.Join(EncoreRepoDir, "go.mod")); err != nil {
		RuntimeDir = env.EncoreRuntimesPath()
	}
}

func repoDir() string {
	if dir := os.Getenv(EnvRepoDirOverride); dir != "" {
		return dir
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	// nosemgrep
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("unable to get repo directory: %s", out))
	}
	return string(bytes.TrimSpace(out))
}
