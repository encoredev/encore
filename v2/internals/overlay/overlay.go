package overlay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"encr.dev/pkg/paths"
)

// File describes a file to generate or rewrite.
type File struct {
	// Source is where on the filesystem the original file (in the case of a rewrite)
	// or where the generated file should be overlaid into.
	Source paths.FS

	// Contents are the file contents of the overlaid file.
	Contents []byte

	// If set, overrides Contents and uses the dest directly instead.
	Dest paths.FS
}

// Write writes the overlay files to the workdir
// and generates an overlay.json file suitable for passing to 'go build -overlay'.
// It returns the path to the written overlay.json file (which lives within workdir).
func Write(workdir paths.FS, files []File) (overlayFile paths.FS, err error) {
	overlay := make(map[string]string) // src -> dst
	seenNames := make(map[string]bool)

	addFile := func(f File) error {
		src := f.Source.ToIO()
		if f.Dest != "" {
			overlay[src] = f.Dest.ToIO()
			return nil
		}

		baseName := "gen_" + filepath.Base(filepath.Dir(src)) + "__" + filepath.Base(src)
		if _, exists := overlay[src]; exists {
			return fmt.Errorf("duplicate overlay of %s", src)
		}

		// Compute a reasonable destination name for the file.
		ext := filepath.Ext(baseName)
		nameWithoutExt := strings.TrimSuffix(baseName, ext)

		// Keep generating names until we get one that doesn't conflict.
		candidate := baseName
		for i := 1; seenNames[candidate]; i++ {
			candidate = fmt.Sprintf("%s_%d%s", nameWithoutExt, i, ext)
		}
		seenNames[candidate] = true
		dst := workdir.Join(candidate)

		// Write the file.
		overlay[src] = dst.ToIO()
		if err := os.WriteFile(dst.ToIO(), f.Contents, 0644); err != nil {
			return fmt.Errorf("write overlay file %s: %v", dst, err)
		}
		return nil
	}

	for _, f := range files {
		if err := addFile(f); err != nil {
			return "", err
		}
	}

	// Compute the overlay.json data.
	data, _ := json.Marshal(map[string]any{"Replace": overlay})
	overlayFile = workdir.Join("overlay.json")
	if err := os.WriteFile(overlayFile.ToIO(), data, 0644); err != nil {
		return "", fmt.Errorf("write overlay file %s: %v", overlayFile, err)
	}
	return overlayFile, nil
}
