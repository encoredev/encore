package apps

import (
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rjeczalik/notify"
	"github.com/rs/zerolog/log"
	"go4.org/syncutil"

	"encr.dev/cli/daemon/internal/manifest"
	"encr.dev/cli/internal/appfile"
)

var ErrNotFound = errors.New("app not found")

func NewManager(db *sql.DB) *Manager {
	return &Manager{
		db:        db,
		instances: make(map[string]*Instance),
	}
}

// Manager keeps track of known apps and watches them for changes.
type Manager struct {
	db         *sql.DB
	setupWatch syncutil.Once

	appRegMu     sync.Mutex
	appListeners []func(*Instance)

	watchMu  sync.Mutex
	watchers []WatchFunc

	instanceMu sync.Mutex
	instances  map[string]*Instance // root -> instance
}

// Track begins tracking an app, and marks it as updated
// if the app is already tracked.
func (mgr *Manager) Track(appRoot string) (*Instance, error) {
	app, err := mgr.resolve(appRoot)
	if err != nil {
		return nil, err
	}
	_, err = mgr.db.Exec(`
		INSERT OR REPLACE INTO app (root, local_id, platform_id, updated_at)
		VALUES (?, ?, ?, ?)
	`, app.root, app.localID, app.platformID, time.Now())
	if err != nil {
		return nil, errors.Wrap(err, "update app store")
	}
	log.Info().Str("app_id", app.PlatformOrLocalID()).Msg("tracking app")
	return app, nil
}

// FindLatestByPlatformID finds the most recently updated app instance with the given platformID.
// If no such app is found it reports an error matching ErrNotFound.
func (mgr *Manager) FindLatestByPlatformID(platformID string) (*Instance, error) {
	var root string
	err := mgr.db.QueryRow(`
		SELECT root
		FROM app
		WHERE platform_id = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, platformID).Scan(&root)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.WithStack(ErrNotFound)
	} else if err != nil {
		return nil, errors.Wrap(err, "query app store")
	}

	return mgr.resolve(root)
}

// List lists all known apps.
func (mgr *Manager) List() ([]*Instance, error) {
	rows, err := mgr.db.Query(`SELECT root FROM app`)
	if err != nil {
		return nil, errors.Wrap(err, "query apps")
	}
	defer rows.Close()

	var apps []*Instance
	for rows.Next() {
		var root string
		if err := rows.Scan(&root); err != nil {
			return nil, errors.Wrap(err, "scan row")
		}
		app, err := mgr.resolve(root)
		if errors.Is(err, fs.ErrNotExist) {
			log.Debug().Str("root", root).Msg("app no longer exists, skipping")
			continue
		} else if err != nil {
			log.Error().Err(err).Str("root", root).Msg("unable to resolve app")
			continue
		}
		apps = append(apps, app)
	}

	err = errors.Wrap(rows.Err(), "iterate rows")
	return apps, err
}

// RegisterAppListener registers a callback that gets invoked every time
// an app is tracked.
func (mgr *Manager) RegisterAppListener(fn func(*Instance)) {
	mgr.instanceMu.Lock()
	defer mgr.instanceMu.Unlock()

	mgr.appRegMu.Lock()
	mgr.appListeners = append(mgr.appListeners, fn)
	mgr.appRegMu.Unlock()

	// Call the handler for all existing apps
	for _, inst := range mgr.instances {
		fn(inst)
	}
}

// WatchFunc is the signature of functions registered as app watchers.
type WatchFunc func(*Instance, notify.EventInfo)

// WatchAll watches all apps for changes.
func (mgr *Manager) WatchAll(fn WatchFunc) error {
	err := mgr.setupWatch.Do(func() error {
		// Begin tracking all known apps by calling List (since it calls resolve).
		_, err := mgr.List()
		return err
	})
	if err != nil {
		return err
	}

	mgr.watchMu.Lock()
	mgr.watchers = append(mgr.watchers, fn)
	mgr.watchMu.Unlock()
	return nil
}

func (mgr *Manager) onWatchEvent(i *Instance, ev notify.EventInfo) {
	mgr.watchMu.Lock()
	watchers := mgr.watchers
	mgr.watchMu.Unlock()
	for _, fn := range watchers {
		fn(i, ev)
	}
}

// resolve resolves the current information about the app located at appRoot.
// If the app does not exist (either because appRoot does not exist,
// or because encore.app does not exist within it), it reports an error
// matching fs.ErrNotExist.
func (mgr *Manager) resolve(appRoot string) (*Instance, error) {
	mgr.instanceMu.Lock()
	defer mgr.instanceMu.Unlock()

	if existing, ok := mgr.instances[appRoot]; ok {
		return existing, nil
	}

	// Parse the encore.app file
	var encore *appfile.File
	{
		path := filepath.Join(appRoot, appfile.Name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		encore, err = appfile.Parse(data)
		if err != nil {
			return nil, errors.Wrap(err, "parse encore.app")
		}
	}

	// Parse the manifest file
	man, err := manifest.ReadOrCreate(appRoot)
	if err != nil {
		return nil, errors.Wrap(err, "parse manifest")
	}

	i := NewInstance(appRoot, man.LocalID, encore.ID)
	i.mgr = mgr
	if err := i.beginWatch(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Error().Err(err).Str("id", i.PlatformOrLocalID()).Msg("unable to begin watching app")
	}
	mgr.instances[appRoot] = i

	// Notify any listeners about the new app
	for _, fn := range mgr.appListeners {
		fn(i)
	}

	return i, nil
}

// Instance describes an app instance known by the Encore daemon.
type Instance struct {
	root       string
	localID    string
	platformID string

	// mgr is a reference to the manager that created it.
	// It may be nil if an instance was created without a manager.
	mgr *Manager

	setupWatch syncutil.Once
	watchMu    sync.Mutex
	watchers   []WatchFunc
}

func NewInstance(root, localID, platformID string) *Instance {
	return &Instance{root: root, localID: localID, platformID: platformID}
}

// Root returns the filesystem path for the app root.
// It always returns a non-empty string.
func (i *Instance) Root() string { return i.root }

// LocalID reports a local, random id unique for this app,
// as persisted in the .encore/manifest.json file.
// It always returns a non-empty string.
func (i *Instance) LocalID() string { return i.localID }

// PlatformID reports the Encore Platform's ID for this app.
// If the app is not linked it reports the empty string.
func (i *Instance) PlatformID() string { return i.platformID }

// PlatformOrLocalID reports PlatformID() if set and otherwise LocalID().
func (i *Instance) PlatformOrLocalID() string {
	if i.platformID != "" {
		return i.platformID
	}
	return i.localID
}

func (i *Instance) Watch(fn WatchFunc) error {
	if err := i.beginWatch(); err != nil {
		return err
	}

	i.watchMu.Lock()
	i.watchers = append(i.watchers, fn)
	i.watchMu.Unlock()
	return nil
}

func (i *Instance) beginWatch() error {
	return i.setupWatch.Do(func() error {
		evs := make(chan notify.EventInfo, 100)
		err := notify.Watch(filepath.Join(i.root, "..."), evs, notify.All)
		if err != nil {
			return errors.Wrap(err, "watch")
		}

		go func() {
			for ev := range evs {
				if i.mgr != nil {
					i.mgr.onWatchEvent(i, ev)
				}

				i.watchMu.Lock()
				watchers := i.watchers
				i.watchMu.Unlock()
				for _, fn := range watchers {
					fn(i, ev)
				}
			}
		}()

		return nil
	})
}
