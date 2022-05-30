package apps

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog/log"

	"encr.dev/cli/daemon/internal/manifest"
	"encr.dev/cli/internal/appfile"
)

var ErrNotFound = errors.New("app not found")

// Instance describes an app instance known by the Encore daemon.
type Instance struct {
	root       string
	localID    string
	platformID string
}

func NewInstance(root, localID, platformID string) *Instance {
	return &Instance{root, localID, platformID}
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

func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db, instances: make(map[string]*Instance)}
}

// Manager keeps track of known apps and watches them for changes.
type Manager struct {
	db *sql.DB

	mu        sync.Mutex
	instances map[string]*Instance // root -> instance
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

// resolve resolves the current information about the app located at appRoot.
// If the app does not exist (either because appRoot does not exist,
// or because encore.app does not exist within it), it reports an error
// matching fs.ErrNotExist.
func (mgr *Manager) resolve(appRoot string) (*Instance, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

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

	i := &Instance{
		root:       appRoot,
		localID:    man.LocalID,
		platformID: encore.ID,
	}
	mgr.instances[appRoot] = i
	return i, nil
}
