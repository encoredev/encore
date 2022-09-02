package vfs

import (
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"
)

func TestFromDir(t *testing.T) {
	c := qt.New(t)

	dir, err := FromDir(
		filepath.Join(".", "testdata", "filteredglob"),
		func(file string) bool { return filepath.Ext(file) == ".json" },
	)
	c.Assert(err, qt.IsNil, qt.Commentf("error creating VFS"))

	// Test that the VFS contains the expected number of files (no more no less)
	fileCount := 0
	dirCount := 0
	err = fs.WalkDir(dir, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileCount++
		} else {
			dirCount++
		}
		return nil
	})
	c.Assert(err, qt.IsNil, qt.Commentf("error walking directory"))
	c.Assert(fileCount, qt.Equals, 3, qt.Commentf("unexpected number of files"))
	c.Assert(dirCount, qt.Equals, 4, qt.Commentf("unexpected number of directories"))

	// Perform the standardised tests on the VFS implementation checking for the existance of the files we wanted
	if err := fstest.TestFS(dir, "blahsvc/another.json", "blahsvc/test.json", "foosystem/barservice/blah.json"); err != nil {
		t.Fatal(err)
	}
}
