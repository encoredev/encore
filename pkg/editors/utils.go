package editors

import (
	"errors"
	"io/fs"
	"os"
)

// Checks if the given path exists.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}
