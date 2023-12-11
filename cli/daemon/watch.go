package daemon

import (
	"bufio"
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bep/debounce"
	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog/log"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/run"
	"encr.dev/pkg/watcher"
)

func (s *Server) watchApps() {
	if os.Getenv("ENCORE_DAEMON_WATCH") == "0" {
		return
	}
	s.apps.RegisterAppListener(func(i *apps.Instance) {
		s.regenerateUserCode(context.Background(), i)
		if err := s.updateGitIgnore(i); err != nil {
			log.Error().Err(err).Msg("unable to update app gitignore")
		}
	})
	if err := s.apps.WatchAll(s.onWatchEvent); err != nil {
		log.Error().Err(err).Msg("unable to set up app watchers")
	} else {
		log.Info().Msg("successfully set up file watchers")
	}
}

func (s *Server) onWatchEvent(i *apps.Instance, events []watcher.Event) {
	if run.IgnoreEvents(events) {
		return
	}

	// Use debounce to avoid calling this on every single change.
	s.appDebounceMu.Lock()
	deb := s.appDebouncers[i]
	if deb == nil {
		deb = &regenerateCodeDebouncer{
			debounce: debounce.New(100 * time.Millisecond),
			doRun:    func() { s.regenerateUserCode(context.Background(), i) },
		}
		s.appDebouncers[i] = deb
	}
	s.appDebounceMu.Unlock()

	deb.ChangeEvent()
}

type regenerateCodeDebouncer struct {
	debounce func(func())
	mu       sync.Mutex
	running  bool
	runAfter bool

	doRun func()
}

func (g *regenerateCodeDebouncer) ChangeEvent() {
	g.debounce(func() {
		g.mu.Lock()

		// If we're already running, mark to run again when complete.
		if g.running {
			g.runAfter = true
			g.mu.Unlock()
			return
		}

		// Otherwise, keep re-running for as long as change events come in.
		g.running = true
		g.runAfter = true // to start us off, at least once.
		for g.runAfter {
			g.runAfter = false // reset for next time
			g.mu.Unlock()
			g.doRun() // actually run
			g.mu.Lock()
		}

		// If we get here g.runAfter nobody requested another run, so we can stop.
		g.running = false

		g.mu.Unlock()
	})
}

func (s *Server) regenerateUserCode(ctx context.Context, app *apps.Instance) {
	if err := s.genUserFacing(ctx, app); err != nil {
		log.Error().Err(err).Str("app", app.PlatformOrLocalID()).Msg("failed to regenerate app")
	} else {
		log.Info().Str("app", app.PlatformOrLocalID()).Msg("successfully generated user code")
	}
}

// updateGitIgnore updates the gitignore file to include Encore directives, if needed.
func (s *Server) updateGitIgnore(i *apps.Instance) error {
	dst := filepath.Join(i.Root(), ".gitignore")
	data, err := os.ReadFile(dst)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errors.Wrap(err, "read .gitignore")
	}

	// Find which directives are already present
	directives := []string{"encore.gen.go", "encore.gen.cue", "/.encore"}
	found := make([]bool, len(directives))
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		ln := scanner.Text()
		for i, directive := range directives {
			if ln == directive {
				found[i] = true
			}
		}
	}

	// Add the ones that are missing
	updated := false
	for i, directive := range directives {
		if !found[i] {
			if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
				data = append(data, '\n')
			}
			data = append(data, directive+"\n"...)
			updated = true
		}
	}

	// Write the file back if there were any changes
	if updated {
		return os.WriteFile(dst, data, 0644)
	}
	return nil
}
