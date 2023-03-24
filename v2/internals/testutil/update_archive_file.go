package testutil

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rogpeppe/go-internal/renameio"
	"github.com/rogpeppe/go-internal/testscript"
	"github.com/rogpeppe/go-internal/txtar"
)

func UpdateArchiveFile(ts *testscript.TestScript, sourceDir string, filename string, content string) {
	wd := ts.Getenv("WORK")

	// Work out the archive name from the work directory
	// testscript always creates a directory with the name `script-<archive name>`
	baseName := filepath.Base(wd)
	if !strings.HasPrefix(baseName, "script-") {
		ts.Fatalf("unable to identify archvie name from work directory `%s`", baseName)
	}
	archiveName := strings.TrimPrefix(baseName, "script-")

	// Check the possible extensions
	foundExt := false
	possibleExts := []string{".txt", ".txtar"}
	for _, ext := range possibleExts {
		archivePath := filepath.Join(sourceDir, archiveName+ext)
		if _, err := os.Stat(archivePath); err == nil {
			archiveName = archiveName + ext
			foundExt = true
			break
		}
	}
	if !foundExt {
		ts.Fatalf("unable to identify archive file from work directory `%s/%s`", sourceDir, archiveName)
	}

	ts.Logf("Updating archive file `%s` with expected file `%s`...", archiveName, filename)

	// Read the archive
	archivePath := filepath.Join(sourceDir, archiveName)
	ar, err := txtar.ParseFile(archivePath)
	if err != nil {
		ts.Fatalf("unable to read archive file `%s`: %v", archiveName, err)
	}

	// Update the archive
	fileFound := false
	for i, f := range ar.Files {
		if f.Name == filename {
			ar.Files[i].Data = []byte(content)
			fileFound = true
			break
		}
	}
	if !fileFound {
		ar.Files = append(ar.Files, txtar.File{Name: filename, Data: []byte(content)})
	}

	// Write the archive
	err = renameio.WriteFile(archivePath, txtar.Format(ar))
	if err != nil {
		ts.Fatalf("unable to write archive file `%s`: %v", archiveName, err)
	}
}
