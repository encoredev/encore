package overlay

import "encr.dev/v2/internal/paths"

// File describes a file to generate or rewrite.
type File struct {
	// Source is where on the filesystem the original file (in the case of a rewrite)
	// or where the generated file should be overlaid into.
	Source paths.FS

	// Contents are the file contents of the overlaid file.
	Contents []byte
}
