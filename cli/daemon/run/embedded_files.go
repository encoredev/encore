package run

import (
	"encr.dev/pkg/watcher"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var embeddedFiles = make(map[string][]string)

// ignoreEventEmbedded checks whether the event is related to an embedded file
func ignoreEventEmbedded(event watcher.Event) (bool, error) {
	switch event.EventType {
	case watcher.CREATED:
		return true, handleCreatedFile(event.Path)
	case watcher.DELETED:
		return true, handleDeletedFile(event.Path)
	case watcher.MODIFIED:
		return handleModifiedFile(event.Path)
	default:
		return true, nil
	}
}

func handleModifiedFile(path string) (bool, error) {
	if strings.HasSuffix(path, ".go") {
		return true, updateEmbeddedFiles(path)
	}

	embedded, err := isFileEmbedded(path)
	if err != nil {
		return true, err
	}

	return !embedded, nil
}

func initializeEmbeddedFilesTracker(root string) error {
	if len(embeddedFiles) > 0 {
		return nil
	}

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		return updateEmbeddedFiles(path)
	})
}

func handleCreatedFile(path string) error {
	if strings.HasSuffix(path, ".go") {
		return updateEmbeddedFiles(path)
	}
	return nil
}

func handleDeletedFile(path string) error {
	delete(embeddedFiles, path)
	return nil
}

func isFileEmbedded(fpath string) (bool, error) {
	for _, files := range embeddedFiles {
		for _, file := range files {
			if file == fpath {
				return true, nil
			}
		}
	}
	return false, nil
}

func updateEmbeddedFiles(path string) error {
	embeds, err := parseEmbeddedFiles(path)
	if err != nil {
		return fmt.Errorf("failed to parse embedded files: %w", err)
	}
	embeddedFiles[path] = embeds
	return nil
}

// parseEmbeddedFiles returns all the embedded files for a given source file
func parseEmbeddedFiles(sourceFile string) ([]string, error) {
	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}

	var embeddedPaths []string
	sourceDir := filepath.Dir(sourceFile)
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "//go:embed") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				dir := parts[1]
				filepaths, err := getFilePathsFromDir(filepath.Join(sourceDir, dir))
				if err != nil {
					return nil, err
				}
				embeddedPaths = append(embeddedPaths, filepaths...)
			}
		}
	}
	return embeddedPaths, nil
}

// getFilePathsFromDir retrieves all file paths from a directory recursively
func getFilePathsFromDir(dir string) ([]string, error) {
	var filePaths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		filePaths = append(filePaths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk through directory %s: %w", dir, err)
	}
	return filePaths, nil
}
