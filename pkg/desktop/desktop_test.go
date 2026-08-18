package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// badPaths are paths no machine should act on, whether or not it has a desktop.
func badPaths(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"empty":    "",
		"relative": filepath.Join("photos", "photo.png"),
		"missing":  filepath.Join(t.TempDir(), "gone.png"),
	}
}

// TestCheckPath covers path validation on its own, so it runs the same on a
// developer's machine and on a headless CI machine, where Open and Reveal turn
// every path away with ErrNoDesktop before looking at it.
func TestCheckPath(t *testing.T) {
	for name, path := range badPaths(t) {
		if err := checkPath(path); err == nil {
			t.Errorf("checkPath(%q) [%s] = nil, want an error", path, name)
		}
	}

	existing := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(existing, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkPath(existing); err != nil {
		t.Errorf("checkPath(%q) = %v, want nil", existing, err)
	}
	// A directory is as valid a target as a file.
	if err := checkPath(filepath.Dir(existing)); err != nil {
		t.Errorf("checkPath(%q) = %v, want nil", filepath.Dir(existing), err)
	}
}

// TestOpenAndRevealRejectBadPaths pins that neither ever launches anything for a
// path that can't be acted on. It asserts only that an error comes back, since
// which one depends on whether the machine running the test has a desktop.
func TestOpenAndRevealRejectBadPaths(t *testing.T) {
	for name, path := range badPaths(t) {
		for op, fn := range map[string]func(string) error{"Open": Open, "Reveal": Reveal} {
			if err := fn(path); err == nil {
				t.Errorf("%s(%q) [%s] = nil, want an error", op, path, name)
			}
		}
	}
}

// TestUnsupportedMachine covers a machine with no desktop, which is what CI is:
// a path that is perfectly good still can't be opened, and says why.
func TestUnsupportedMachine(t *testing.T) {
	if Supported() {
		t.Skip("this machine has a desktop, so Open would launch a program")
	}

	existing := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(existing, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for op, fn := range map[string]func(string) error{"Open": Open, "Reveal": Reveal} {
		if err := fn(existing); !errors.Is(err, ErrNoDesktop) {
			t.Errorf("%s(%q) = %v, want ErrNoDesktop", op, existing, err)
		}
	}
}
