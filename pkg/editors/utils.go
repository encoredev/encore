package editors

import (
	"os"
)

// Checks if the given path exists.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
