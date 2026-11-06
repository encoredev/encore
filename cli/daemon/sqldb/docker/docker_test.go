package docker

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestIsDockerRunning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker command is a shell script")
	}

	tests := []struct {
		name     string
		exitCode int
		want     bool
	}{
		{name: "daemon reachable", exitCode: 0, want: true},
		{name: "daemon not reachable", exitCode: 1, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			dir := t.TempDir()
			argsFile := filepath.Join(dir, "args")
			script := "#!/bin/sh\necho \"$@\" > " + argsFile + "\nexit " + strconv.Itoa(tt.exitCode) + "\n"
			c.Assert(os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o755), qt.IsNil)
			t.Setenv("PATH", dir)

			c.Assert(isDockerRunning(context.Background()), qt.Equals, tt.want)

			args, err := os.ReadFile(argsFile)
			c.Assert(err, qt.IsNil)
			c.Assert(string(args), qt.Equals, "version\n")
		})
	}
}
