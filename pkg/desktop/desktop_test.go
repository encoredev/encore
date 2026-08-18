package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestOpenRejectsBadPaths covers the checks that run before anything is handed to
// the operating system. The success path can't be tested: it opens a window.
func TestOpenRejectsBadPaths(t *testing.T) {
	existing := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(existing, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"empty", ""},
		{"relative", filepath.Join("photos", "photo.png")},
		{"missing", filepath.Join(t.TempDir(), "gone.png")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for name, op := range map[string]func(string) error{"Open": Open, "Reveal": Reveal} {
				if err := op(test.path); err == nil {
					t.Errorf("%s(%q) = nil, want an error", name, test.path)
				} else if errors.Is(err, ErrNoDesktop) {
					// The path checks run first precisely so that they are what
					// gets reported, on a build machine as much as a developer's.
					t.Errorf("%s(%q) = %v, want a path error", name, test.path, err)
				}
			}
		})
	}

	// A path that is fine but a machine that has nowhere to open it: reported as
	// ErrNoDesktop so a caller can tell "impossible here" from "bad input".
	if !Supported() {
		if err := Open(existing); !errors.Is(err, ErrNoDesktop) {
			t.Errorf("Open(%q) = %v, want ErrNoDesktop", existing, err)
		}
	}
}
