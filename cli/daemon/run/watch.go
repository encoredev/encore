package run

import (
	"path/filepath"

	"github.com/rjeczalik/notify"
)

// watch watches the given app for changes, and reports
// them on c.
func (mgr *Manager) watch(run *Run) error {
	evs := make(chan notify.EventInfo)
	if err := notify.Watch(filepath.Join(run.Root, "..."), evs, notify.All); err != nil {
		return err
	}

	go func() {
		<-run.Done()
		notify.Stop(evs)
	}()

	go func() {
		for {
			select {
			case <-run.Done():
				return
			case ev := <-evs:
				if ignoreEvent(run.Root, ev) {
					continue
				}
				mgr.runStdout(run, []byte("Changes detected, recompiling...\n"))
				if _, err := run.Reload(); err != nil {
					mgr.runStderr(run, []byte(err.Error()))
				} else {
					mgr.runStdout(run, []byte("Reloaded successfully.\n"))
				}
			}
		}
	}()
	return nil
}

func ignoreEvent(appRoot string, ev notify.EventInfo) bool {
	// Ignore events inside .git directories
	path := ev.Path()
	for {
		if filepath.Base(path) == ".git" {
			return true
		}
		dir := filepath.Dir(path)
		if path == dir {
			return false
		}
		path = dir
	}
}
