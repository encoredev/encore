package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/watcher"
)

// watch watches the given app for changes, and reports
// them on c.
func (mgr *Manager) watch(run *Run) error {
	sub, err := run.App.Watch(func(i *apps.Instance, event []watcher.Event) {
		if IgnoreEvents(run.App.Root(), event) {
			return
		}

		mgr.RunStdout(run, []byte("Changes detected, recompiling...\n"))
		if err := run.Reload(); err != nil {
			if errList := AsErrorList(err); errList != nil {
				mgr.RunError(run, errList)
			} else {
				errStr := err.Error()
				if !strings.HasSuffix(errStr, "\n") {
					errStr += "\n"
				}
				mgr.RunStderr(run, []byte(errStr))
			}
		} else {
			mgr.RunStdout(run, []byte("Reloaded successfully.\n"))
		}
	})
	if err != nil {
		return err
	}

	go func() {
		<-run.Done()
		run.App.Unwatch(sub)
	}()

	return nil
}

// IgnoreEvents will return true if _all_ events are on files that should be ignored
// as they do not impact the running app, or are the result of Encore itself generating code.
func IgnoreEvents(root string, events []watcher.Event) bool {
	for _, event := range events {
		if !ignoreEvent(root, event) {
			return false
		}
	}
	return true
}

func ignoreEvent(root string, ev watcher.Event) bool {
	filename := filepath.Base(ev.Path)
	if strings.HasPrefix(strings.ToLower(filename), "encore.gen.") {
		// Ignore generated code
		return true
	}

	embedded, err := checkIfFileIsEmbedded(root, ev.Path)
	if err != nil {
		return true
	}
	if embedded {
		// Don't ignore embedded files
		return false
	}

	// Ignore files which wouldn't impact the running app
	ext := filepath.Ext(ev.Path)
	switch ext {
	case ".go", ".sql", ".mod", ".sum", ".work", ".app", ".cue",
		".ts", ".js", ".tsx", ".jsx", ".mts", ".mjs", ".cjs", ".cts":
		return false
	default:
		return true
	}
}

// checkIfFileIsEmbedded checks if a file is embedded somewhere in the encore project using the //go:embed directive
func checkIfFileIsEmbedded(projectRoot, sourceFile string) (bool, error) {
	isEmbedded := false

	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			embeddedPaths, err := getEmbeddedFilePaths(path)
			if err != nil {
				return err
			}
			for _, embeddedPath := range embeddedPaths {
				if embeddedPath == sourceFile {
					isEmbedded = true
					return nil
				}
			}
		}
		return nil
	})

	return isEmbedded, err
}

func getEmbeddedFilePaths(sourceFile string) ([]string, error) {
	var embeddedPaths []string

	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "//go:embed") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				dir := parts[1]
				filepaths, err := getFilePathsFromDir(filepath.Join(filepath.Dir(sourceFile), dir))
				if err != nil {
					return nil, err
				}
				embeddedPaths = append(embeddedPaths, filepaths...)
			}
		}
	}

	return embeddedPaths, nil
}

func getFilePathsFromDir(dir string) ([]string, error) {
	var filePaths []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			filePaths = append(filePaths, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk through directory %s: %w", dir, err)
	}

	return filePaths, nil
}
