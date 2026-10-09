package run

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/logrusorgru/aurora/v3"

	"encr.dev/cli/daemon/run/infra"
	meta "encr.dev/proto/encore/parser/meta/v1"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/watcher"
)

// watch watches the given app for changes, keeping generated code fresh for
// as long as run is alive. liveReload controls whether we additionally
// reload the running app on changes - this is what `--watch=false` disables;
// it doesn't stop us from watching altogether, since the app's files should
// stay watched regardless so codegen doesn't go stale while `encore run` is
// up.
func (mgr *Manager) watch(run *Run, liveReload bool) error {
	sub, err := run.App.Watch(func(i *apps.Instance, event []watcher.Event) {
		if !liveReload || IgnoreEvents(event) {
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
			mgr.RunStdout(run, []byte(run.reloadedMessage()))
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

// reloadedMessage returns the message to print after a successful reload.
//
// Database changes are only applied when the SQL cluster starts, so if the
// reload added databases or migrations, it says so instead of reporting
// success. The full list is only printed when it changes, so a burst of
// reloads while setting up a database doesn't repeat it. Databases that the
// reload did set up (e.g. the app's first database, which starts the cluster)
// are reported too. If a database can't be checked, that's reported instead
// of claiming success, and the previously reported list is kept.
func (r *Run) reloadedMessage() string {
	var (
		pending   []infra.PendingDBChange
		upToDate  []string
		checkErrs []infra.DBCheckError
		md        *meta.Data
	)
	if p := r.ProcGroup(); p != nil {
		md = p.Meta
		ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
		pending, upToDate, checkErrs = r.ResourceManager.DBChanges(ctx, md)
		cancel()
	}
	msg := formatPendingDBChanges(pending)

	r.dbStateMu.Lock()
	prev := r.lastPendingDBMsg
	if len(checkErrs) == 0 {
		r.lastPendingDBMsg = msg
	}
	var created []string
	for _, name := range upToDate {
		if !r.knownDBs[name] {
			created = append(created, formatCreatedDB(md, name))
			if r.knownDBs == nil {
				r.knownDBs = make(map[string]bool)
			}
			r.knownDBs[name] = true
		}
	}
	r.dbStateMu.Unlock()

	var b strings.Builder
	for _, line := range created {
		b.WriteString(aurora.Green(line).String() + "\n")
	}
	for _, e := range checkErrs {
		line := fmt.Sprintf("Could not check database %q for pending migrations: %v", e.DBName, e.Err)
		b.WriteString(aurora.Yellow(line).String() + "\n")
	}
	switch {
	case msg == "" && len(checkErrs) > 0:
		b.WriteString("Reloaded.\n")
	case msg == "":
		b.WriteString("Reloaded successfully.\n")
	case msg == prev:
		b.WriteString(aurora.Yellow("Reloaded, database changes still pending. Restart encore run to apply them.").String() + "\n")
	default:
		b.WriteString(aurora.Yellow("Reloaded, but database changes need a restart to apply:").String() + "\n" +
			msg +
			aurora.Yellow("Restart encore run to apply them.").String() + "\n")
	}
	return b.String()
}

// formatCreatedDB describes a database that was set up during a reload.
func formatCreatedDB(md *meta.Data, name string) string {
	var n int
	for _, db := range md.SqlDatabases {
		if db.Name == name {
			n = len(db.Migrations)
		}
	}
	switch n {
	case 0:
		return fmt.Sprintf("Created database %q.", name)
	case 1:
		return fmt.Sprintf("Created database %q and applied 1 migration.", name)
	default:
		return fmt.Sprintf("Created database %q and applied %d migrations.", name, n)
	}
}

// formatPendingDBChanges formats changes as one indented line per database.
func formatPendingDBChanges(changes []infra.PendingDBChange) string {
	var b strings.Builder
	for _, c := range changes {
		switch {
		case c.New && len(c.Migrations) == 0:
			fmt.Fprintf(&b, "  new database %q (no migrations yet)\n", c.DBName)
		case c.New:
			fmt.Fprintf(&b, "  new database %q: %s\n", c.DBName, strings.Join(c.Migrations, ", "))
		default:
			fmt.Fprintf(&b, "  database %q: %s not applied\n", c.DBName, strings.Join(c.Migrations, ", "))
		}
	}
	return b.String()
}

// IgnoreEvents will return true if _all_ events are on files that should be ignored
// as the do not impact the running app, or are the result of Encore itself generating code.
func IgnoreEvents(events []watcher.Event) bool {
	for _, event := range events {
		if !ignoreEvent(event) {
			return false
		}
	}
	return true
}

func ignoreEvent(ev watcher.Event) bool {
	filename := filepath.Base(ev.Path)
	if strings.HasPrefix(strings.ToLower(filename), "encore.gen.") {
		// Ignore generated code
		return true
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
