package apps

import (
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog/log"
	"go4.org/syncutil"

	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/internal/manifest"
	"encr.dev/internal/conf"
	"encr.dev/internal/env"
	"encr.dev/internal/goldfish"
	"encr.dev/pkg/appfile"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/watcher"
	"encr.dev/pkg/xos"
	meta "encr.dev/proto/encore/parser/meta/v1"
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
	`, app.root, app.localID, app.PlatformID(), time.Now())
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

func (mgr *Manager) FindLatestByPlatformOrLocalID(id string) (*Instance, error) {
	// Local ID do not contain hyphens, platform ID's always contain hyphens.
	if strings.Contains(id, "-") {
		return mgr.FindLatestByPlatformID(id)
	}

	var root string
	err := mgr.db.QueryRow(`
		SELECT root
		FROM app
		WHERE local_id = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, id).Scan(&root)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.WithStack(ErrNotFound)
	} else if err != nil {
		return nil, errors.Wrap(err, "query app store")
	}

	return mgr.resolve(root)
}

// List lists all known apps.
func (mgr *Manager) List() ([]*Instance, error) {
	roots, err := mgr.listRoots()
	if err != nil {
		return nil, err
	}

	var apps []*Instance
	for _, root := range roots {
		app, err := mgr.resolve(root)
		if errors.Is(err, fs.ErrNotExist) {
			log.Debug().Str("root", root).Msg("app no longer exists, skipping")
			// Delete the
			_, _ = mgr.db.Exec(`DELETE FROM app WHERE root = ?`, root)
			continue
		} else if err != nil {
			log.Error().Err(err).Str("root", root).Msg("unable to resolve app")
			continue
		}
		apps = append(apps, app)
	}

	return apps, nil
}

func (mgr *Manager) listRoots() ([]string, error) {
	rows, err := mgr.db.Query(`SELECT root FROM app`)
	if err != nil {
		return nil, errors.Wrap(err, "query app roots")
	}
	defer fns.CloseIgnore(rows)

	var roots []string
	for rows.Next() {
		var root string
		if err := rows.Scan(&root); err != nil {
			return nil, errors.Wrap(err, "scan row")
		}
		roots = append(roots, root)
	}
	err = errors.Wrap(rows.Err(), "iterate rows")
	return roots, err
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
type WatchFunc func(*Instance, []watcher.Event)

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

func (mgr *Manager) onWatchEvent(i *Instance, ev []watcher.Event) {
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

	platformID, err := readPlatformID(appRoot)
	if err != nil {
		return nil, err
	}

	// Parse the manifest file
	man, err := manifest.ReadOrCreate(appRoot)
	if err != nil {
		return nil, errors.Wrap(err, "parse manifest")
	}

	i := NewInstance(appRoot, man.LocalID, platformID)
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

func (mgr *Manager) Close() error {
	mgr.instanceMu.Lock()
	defer mgr.instanceMu.Unlock()

	for _, inst := range mgr.instances {
		if err := inst.Close(); err != nil {
			log.Err(err).Str("id", inst.PlatformOrLocalID()).Msg("unable to close app instance")
			// do not return an error here as we want to close all instances
		}
	}

	return nil
}

// Instance describes an app instance known by the Encore daemon.
type Instance struct {
	root       string
	localID    string
	platformID *goldfish.Cache[string]

	// mgr is a reference to the manager that created it.
	// It may be nil if an instance was created without a manager.
	mgr     *Manager
	watcher *watcher.Watcher

	setupWatch  syncutil.Once
	watchMu     sync.Mutex
	nextWatchID WatchSubscriptionID
	watchers    map[WatchSubscriptionID]*watchSubscription

	mdMu     sync.Mutex
	cachedMd *meta.Data
}

func NewInstance(root, localID, platformID string) *Instance {
	i := &Instance{
		root:     root,
		localID:  localID,
		watchers: make(map[WatchSubscriptionID]*watchSubscription),
	}
	i.platformID = goldfish.New[string](1*time.Second, i.fetchPlatformID)
	if platformID != "" {
		i.platformID.Set(platformID)
	}
	return i
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
func (i *Instance) PlatformID() string {
	val, _ := i.platformID.Get()
	return val
}

// PlatformOrLocalID reports PlatformID() if set and otherwise LocalID().
func (i *Instance) PlatformOrLocalID() string {
	if id := i.PlatformID(); id != "" {
		return id
	}
	return i.localID
}

// Name returns the platform ID for the app, or if there isn't one
// it returns the folder name the app is in.
func (i *Instance) Name() string {
	if id := i.PlatformID(); id != "" {
		return id
	}

	return filepath.Base(i.root)
}

func (i *Instance) fetchPlatformID() (string, error) {
	return readPlatformID(i.root)
}

func readPlatformID(appRoot string) (string, error) {
	// Parse the encore.app file
	path := filepath.Join(appRoot, appfile.Name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	encore, err := appfile.Parse(data)
	if err != nil {
		return "", errors.Wrap(err, "parse encore.app")
	}
	return encore.ID, nil
}

// Experiments returns the enabled experiments for this app.
//
// Note: we read the app file here instead of a cached value so we
// can detect changes between runs of the compiler if we're in
// watch mode.
func (i *Instance) Experiments(environ []string) (*experiments.Set, error) {
	exp, err := appfile.Experiments(i.root)
	if err != nil {
		return nil, err
	}

	return experiments.FromAppFileAndEnviron(exp, environ)
}

func (i *Instance) Lang() appfile.Lang {
	appFile, err := appfile.ParseFile(filepath.Join(i.root, appfile.Name))
	if err != nil {
		return appfile.LangGo
	}
	return appFile.Lang
}

func (i *Instance) ProcessPerService() bool {
	appFile, err := appfile.ParseFile(filepath.Join(i.root, appfile.Name))
	if err != nil {
		return false
	}
	return appFile.Build.Docker.ProcessPerService
}

// GlobalCORS returns the CORS configuration for the app which
// will be applied against all API gateways into the app
func (i *Instance) GlobalCORS() (appfile.CORS, error) {
	cors, err := appfile.GlobalCORS(i.root)
	if err != nil {
		return appfile.CORS{}, err
	}

	// If there are no Global CORS return the default
	if cors == nil {
		return appfile.CORS{}, nil
	}

	return *cors, nil

}

func (i *Instance) Watch(fn WatchFunc) (WatchSubscriptionID, error) {
	if err := i.beginWatch(); err != nil {
		return 0, err
	}

	i.watchMu.Lock()
	i.nextWatchID++
	id := i.nextWatchID
	i.watchers[id] = &watchSubscription{id, fn}
	i.watchMu.Unlock()
	return id, nil
}

func (i *Instance) Unwatch(id WatchSubscriptionID) {
	i.watchMu.Lock()
	delete(i.watchers, id)
	i.watchMu.Unlock()
}

func (i *Instance) beginWatch() error {
	return i.setupWatch.Do(func() error {
		watch, err := watcher.New(i.PlatformOrLocalID())
		if err != nil {
			return errors.Wrap(err, "unable to create watcher")
		}
		i.watcher = watch

		if err := i.watcher.RecursivelyWatch(i.root); err != nil {
			return errors.Wrap(err, "unable to watch app")
		}

		// If we're in dev mode, we want to watch the runtime
		// too, so we can develop changes to the runtime without
		// needing to restart the application.
		if conf.DevDaemon {
			if err := i.watcher.RecursivelyWatch(env.EncoreRuntimesPath()); err != nil {
				return errors.Wrap(err, "unable to watch runtime")
			}
		}

		go func() {
			for {
				events, ok := i.watcher.WaitForEvents()
				if !ok {
					// We're done watching.
					return
				}

				if i.mgr != nil {
					i.mgr.onWatchEvent(i, events)
				}

				i.watchMu.Lock()
				watchers := i.watchers
				i.watchMu.Unlock()
				for _, sub := range watchers {
					sub.f(i, events)
				}
			}
		}()

		return nil
	})
}

// CachePath returns the path to the cache directory for this app.
// It creates the directory if it does not exist.
func (i *Instance) CachePath() (string, error) {
	cacheDir, err := conf.CacheDir()
	if err != nil {
		return "", errors.Wrap(err, "unable to get encore cache dir")
	}

	// we use local ID to be stable if the app is linked to the platform later
	cacheDir = filepath.Join(cacheDir, i.localID)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", errors.Wrap(err, "unable to create app cache dir")
	}

	return cacheDir, nil
}

// CacheMetadata caches the metadata for this app onto the file system
func (i *Instance) CacheMetadata(md *meta.Data) error {
	i.mdMu.Lock()
	defer i.mdMu.Unlock()

	i.cachedMd = md

	cacheDir, err := i.CachePath()
	if err != nil {
		return err
	}

	data, err := proto.Marshal(md)
	if err != nil {
		return errors.Wrap(err, "unable to marshal metadata")
	}

	err = xos.WriteFile(filepath.Join(cacheDir, "metadata.pb"), data, 0644)
	if err != nil {
		return errors.Wrap(err, "unable to write metadata")
	}

	return nil
}

// CachedMetadata returns the cached metadata for this app, if any
func (i *Instance) CachedMetadata() (*meta.Data, error) {
	i.mdMu.Lock()
	defer i.mdMu.Unlock()

	if i.cachedMd != nil {
		return i.cachedMd, nil
	}

	cacheDir, err := i.CachePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(cacheDir, "metadata.pb"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "unable to read metadata")
	}

	md := &meta.Data{}
	err = proto.Unmarshal(data, md)
	if err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal metadata")
	}

	i.cachedMd = md
	return md, nil
}

func (i *Instance) Close() error {
	if i.watcher != nil {
		return i.watcher.Close()
	}
	return nil
}

type WatchSubscriptionID int64

type watchSubscription struct {
	id WatchSubscriptionID
	f  WatchFunc
}
