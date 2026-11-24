package overlay

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"encr.dev/pkg/paths"
)

// A persistent workdir is rebuilt on every build, so an unchanged generated
// file must be left alone rather than rewritten.
func TestWriteLeavesUnchangedFilesAlone(t *testing.T) {
	workdir := paths.FS(t.TempDir())
	src := paths.FS(filepath.Join(t.TempDir(), "svc", "api.go"))
	files := []File{{Source: src, Contents: []byte("package svc\n")}}

	overlayFile, err := Write(workdir, files)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	dst := workdir.Join("gen_svc__api.go").ToIO()
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	for _, p := range []string{dst, overlayFile.ToIO()} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Write(workdir, files); err != nil {
		t.Fatalf("second write: %v", err)
	}
	for _, p := range []string{dst, overlayFile.ToIO()} {
		if st, err := os.Stat(p); err != nil || !st.ModTime().Equal(old) {
			t.Errorf("%s was rewritten although unchanged", filepath.Base(p))
		}
	}

	files[0].Contents = []byte("package svc\n\nvar X = 1\n")
	if _, err := Write(workdir, files); err != nil {
		t.Fatalf("third write: %v", err)
	}
	if got, err := os.ReadFile(dst); err != nil || string(got) != string(files[0].Contents) {
		t.Errorf("changed contents not written: %q, %v", got, err)
	}
}
