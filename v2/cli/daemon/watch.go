package daemon

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/bep/debounce"
	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog/log"

	"encr.dev/compiler"
	"encr.dev/pkg/watcher"
	"encr.dev/v2/cli/daemon/apps"
	"encr.dev/v2/cli/daemon/run"
)

func (s *Server) watchApps() {
	s.apps.RegisterAppListener(func(i *apps.Instance) {
		s.regenerateUserCode(i)
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
		deb = debounce.New(100 * time.Millisecond)
		s.appDebouncers[i] = deb
	}
	s.appDebounceMu.Unlock()

	deb(func() { s.regenerateUserCode(i) })
}

func (s *Server) regenerateUserCode(i *apps.Instance) {
	if err := compiler.GenUserFacing(i.Root()); err != nil {
		log.Error().Err(err).Str("app", i.PlatformOrLocalID()).Msg("failed to regenerate app")
	} else {
		log.Info().Str("app", i.PlatformOrLocalID()).Msg("successfully generated user code")
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

type debouncer = func(fn func())
