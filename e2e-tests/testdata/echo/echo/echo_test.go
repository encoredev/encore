package echo

import (
	"os"
	"strings"
	"testing"
)

// TestEnvsProvided tests that 'go test' was invoked with the envs we expect.
func TestEnvsProvided(t *testing.T) {
	envs := os.Environ()
	want := map[string]string{
		"FOO": "bar",
		"BAR": "baz",
	}
	for _, env := range envs {
		key, val, _ := strings.Cut(env, "=")
		if wantVal, ok := want[key]; ok {
			if val != wantVal {
				t.Errorf("got env %s=%s, want value %s", key, val, wantVal)
			}
		}
	}
}
