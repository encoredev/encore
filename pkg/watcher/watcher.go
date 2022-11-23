package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Watcher struct {
	mutex sync.Mutex

	log     *zerolog.Logger
	appRoot string

	watcher     *fsnotify.Watcher
	directories map[string]struct{}
	stop        chan struct{}

	EventsReady chan struct{}

	events         *Events
	notifyListener func()
}

func New(appID string) (*Watcher, error) {
	fswatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	logger := log.With().Str("component", "watcher").Str("app", appID).Logger()
	logger.Debug().Msg("File system watcher created")
	w := &Watcher{
		watcher:     fswatcher,
		log:         &logger,
		directories: make(map[string]struct{}),
		stop:        make(chan struct{}),
		events:      nil,
		EventsReady: make(chan struct{}),
	}

	// We debounce this to give the system time to process mass file updates
	d := debounce.New(50 * time.Millisecond)
	w.notifyListener = func() {
		d(func() {
			w.EventsReady <- struct{}{}
		})
	}

	go w.listenForChangeEvents()

	return w, nil
}

func (w *Watcher) RecursivelyWatch(folder string) error {
	folder = filepath.Clean(folder)

	// We don't want to watch certain folders as they'll never impact
	// an Encore app, and they cause an extreme amount of noise.
	folderName := filepath.Base(folder)
	if folderName == "node_modules" {
		return nil
	}

	// Don't watch hidden folders like `.git` or `.idea` as
	// they also don't impact an Encore app.
	if len(folderName) > 1 && folderName[0] == '.' {
		return nil
	}

	w.mutex.Lock()

	// Track the fact we're watching this directory
	if _, found := w.directories[folder]; found {
		w.mutex.Unlock()
		return nil
	}
	w.directories[folder] = struct{}{}
	w.mutex.Unlock() // unlock here to prevent reentrant locks during recursion

	if err := w.watcher.Add(folder); err != nil {
		return err
	}

	return filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return w.RecursivelyWatch(path)
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
			switch {
			case event.Op&fsnotify.Create == fsnotify.Create:
				w.handleCreateEvent(event.Name)
			case event.Op&fsnotify.Write == fsnotify.Write:
				w.handleWriteEvent(event.Name)
			case event.Op&fsnotify.Remove == fsnotify.Remove:
				w.handleDeleteEvent(event.Name)
			}

		case err := <-w.watcher.Errors:
			w.log.Err(err).Msg("Watcher error")
		}
	}
}

func (w *Watcher) handleCreateEvent(path string) {
	if info, err := os.Stat(path); err != nil {
		w.log.Err(err).Str("path", path).Msg("Unable to stat file")
	} else if info.IsDir() {
		if err := w.RecursivelyWatch(path); err != nil {
			w.log.Err(err).Str("path", path).Msg("Unable to start watching new directory")
		}
	} else {
		w.recordEventInBatch(path, CREATED, info)
	}
}

func (w *Watcher) handleDeleteEvent(path string) {
	// If it's a directory we're watching, stop watching it
	w.mutex.Lock()
	delete(w.directories, path)
	w.mutex.Unlock()

	w.recordEventInBatch(path, DELETED, nil)
}

func (w *Watcher) handleWriteEvent(path string) {
	if info, err := os.Stat(path); err != nil {
		w.log.Err(err).Str("path", path).Msg("Unable to stat file")
	} else if !info.IsDir() {
		w.recordEventInBatch(path, MODIFIED, info)
	}
}

func (w *Watcher) recordEventInBatch(path string, event EventType, info os.FileInfo) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.events == nil {
		w.events = newEventBatch()
		w.notifyListener()
	}

	w.events.addEvent(path, event, info)
}

func (w *Watcher) GetEventsBatch() *Events {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	events := w.events
	w.events = nil

	return events
}

func (w *Watcher) Close() error {
	w.stop <- struct{}{}
	close(w.EventsReady)
	close(w.stop)
	return nil
}
