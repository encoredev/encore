package watcher

import (
	"path/filepath"
)

// IgnoreFolder returns true for folders we don't want to watch certain folders
// as they'll never impact an Encore app, and they cause an extreme amount of noise.
func IgnoreFolder(folder string) bool {
	folderName := filepath.Base(filepath.Clean(folder))
	if folderName == "node_modules" || folderName == "encore.gen" {
		return true
	}

	// Don't watch hidden folders like `.git` or `.idea` as
	// they also don't impact an Encore app.
	if len(folderName) > 1 && folderName[0] == '.' {
		return true
	}

	return false
}
