package run

import (
	"path/filepath"
	"strings"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/watcher"
)

// watch watches the given app for changes, and reports
// them on c.
func (mgr *Manager) watch(run *Run) error {

	// Initialize embedded files tracker
	if err := initializeEmbeddedFilesTracker(run.App.Root()); err != nil {
		return err
	}

	sub, err := run.App.Watch(func(i *apps.Instance, event []watcher.Event) {
		if IgnoreEvents(event) {
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
func IgnoreEvents(events []watcher.Event) bool {
	for _, event := range events {
		filename := filepath.Base(event.Path)
		if strings.HasPrefix(strings.ToLower(filename), "encore.gen.") ||
			strings.HasSuffix(filename, "~") {
			// Ignore generated code and temporary files
			return true
		}

		ignore, err := ignoreEventEmbedded(event)
		if err != nil {
			return false
		}

		if !ignoreEvent(event) || !ignore {
			return false
		}
	}
	return true
}

func ignoreEvent(ev watcher.Event) bool {
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
