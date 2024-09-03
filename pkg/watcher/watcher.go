package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encr.dev/pkg/eerror"
)

type Watcher struct {
	mutex          sync.Mutex
	eventCond      *sync.Cond
	events         *Events
	signalDebounce func(func())

	log     *zerolog.Logger
	appRoot string

	watcher     *fsnotify.Watcher
	directories map[string]struct{}
	stop        chan struct{}
}

func New(appID string) (*Watcher, error) {
	fswatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, eerror.Wrap(err, "watcher", "unable to create watcher", map[string]interface{}{"app": appID})
	}

	logger := log.With().Str("component", "watcher").Str("app", appID).Logger()
	logger.Debug().Msg("File system watcher created")
	w := &Watcher{
		watcher:        fswatcher,
		log:            &logger,
		directories:    make(map[string]struct{}),
		stop:           make(chan struct{}),
		events:         nil,
		signalDebounce: debounce.New(50 * time.Millisecond),
	}

	w.eventCond = sync.NewCond(&w.mutex)

	go w.listenForChangeEvents()

	return w, nil
}

func (w *Watcher) RecursivelyWatch(folder string) error {
	return filepath.WalkDir(folder, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			return eerror.Wrap(err, "watcher", "unable to walk directory", map[string]any{"path": path})
		}

		if info.IsDir() {
			folder := filepath.Clean(path)

			if IgnoreFolder(folder) {
				return filepath.SkipDir
			}

			// Track the fact we're watching this directory
			w.mutex.Lock()
			if _, found := w.directories[folder]; found {
				w.mutex.Unlock()
				return filepath.SkipDir
			}
			w.directories[folder] = struct{}{}
			w.mutex.Unlock() // unlock here to prevent reentrant locks during recursion

			// Now start watching this folder
			if err := w.watcher.Add(folder); err != nil {
				return eerror.Wrap(err, "watcher", "unable to add folder to watch", map[string]any{"folder": folder})
			}
		}

		return nil
	})
}

func (w *Watcher) listenForChangeEvents() {
	for {
		select {
		case <-w.stop:
			_ = w.watcher.Close()
			return

		case event := <-w.watcher.Events:
			if event.Has(fsnotify.Remove) {
				w.handleDeleteEvent(event.Name)
			} else if event.Has(fsnotify.Create) {
				w.handleCreateEvent(event.Name)
			} else if event.Has(fsnotify.Write) {
				w.handleWriteEvent(event.Name)
			}

		case err := <-w.watcher.Errors:
			w.log.Err(err).Msg("Watcher error")
		}
	}
}

func (w *Watcher) handleCreateEvent(path string) {
	if info, err := os.Stat(path); err != nil {
		w.log.Err(err).Str("path", path).Msg("unable to stat file")
	} else if info.IsDir() {
		if err := w.RecursivelyWatch(path); err != nil {
			w.log.Err(err).Str("path", path).Msg("unable to start watching new directory")
		}
	} else {
		w.recordEventInBatch(path, CREATED, info)
	}
}

func (w *Watcher) handleDeleteEvent(path string) {
	path = filepath.Clean(path)

	pathWithSep := path + string(filepath.Separator)

	// If it's a directory we're watching, stop watching it
	w.mutex.Lock()
	for watchedFolder := range w.directories {
		// I sthis the path itself, or a subdirectory thereof?
		if strings.HasPrefix(watchedFolder, pathWithSep) || watchedFolder == path {
			if err := w.watcher.Remove(watchedFolder); err != nil {
				w.log.Err(err).Str("path", watchedFolder).Msg("unable to stop watching deleted directory")
			}
			delete(w.directories, watchedFolder)
		}
	}
	w.mutex.Unlock()

	w.recordEventInBatch(path, DELETED, nil)
}

func (w *Watcher) handleWriteEvent(path string) {
	if info, err := os.Stat(path); err != nil {
		w.log.Err(err).Str("path", path).Msg("unable to stat file")
	} else if !info.IsDir() {
		w.recordEventInBatch(path, MODIFIED, info)
	}
}

func (w *Watcher) recordEventInBatch(path string, event EventType, info os.FileInfo) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.events == nil {
		w.events = newEventBatch()
	}

	w.events.addEvent(path, event, info)

	// Debounce the signal to avoid waking up on every event in case of a burst of events.
	w.signalDebounce(func() {
		w.eventCond.Signal()
	})
}

func (w *Watcher) WaitForEvents() (events []Event, ok bool) {
	w.eventCond.L.Lock()
	defer w.eventCond.L.Unlock()

	for {
		select {
		case <-w.stop:
			// We're shutting down, so return immediately.
			return nil, false

		default:
			if w.events == nil || len(w.events.latestEvents) == 0 {
				w.eventCond.Wait()
			}
			// Post-condition: we have at least one event.

			events := w.events.Events()
			w.events = newEventBatch()
			return events, true
		}
	}
}

func (w *Watcher) Close() error {
	close(w.stop)
	return nil
}

func (w *Watcher) Done() <-chan struct{} {
	return w.stop
}
