package tarstream

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/errors"
)

func TestFileSizeChanges(t *testing.T) {
	for _, content := range []string{"ab", "abcdefgh"} {
		t.Run(content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(path, []byte("abcd"), 0644); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			tv := NewTarVec([]Datavec{&PathVec{Path: path, Info: info}, MemVec{Data: []byte("end")}})
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(tv)
			if len(content) < 4 {
				if string(got) != content || !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("truncated: %q, %v", got, err)
				}
			} else if string(got) != "abcdend" || err != nil {
				t.Fatalf("grown: %q, %v", got, err)
			}
			if err := tv.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
