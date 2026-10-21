package cmdutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceRoot(t *testing.T) {
	tests := []struct {
		name string
		git  func(t *testing.T, root string)
	}{
		{"git directory", func(t *testing.T, root string) {
			if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"git file (worktree or submodule)", func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.git(t, root)
			app := filepath.Join(root, "sub", "app")
			if err := os.MkdirAll(app, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, start := range []string{root, app} {
				if got := WorkspaceRoot(start); got != root {
					t.Errorf("WorkspaceRoot(%q) = %q, want %q", start, got, root)
				}
			}
		})
	}
}
