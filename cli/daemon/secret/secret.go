// Package secret fetches and caches development secrets for Encore apps.
package secret

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"github.com/rs/zerolog/log"
	"go4.org/syncutil"
	"golang.org/x/sync/singleflight"

	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/internal/platform"
	"encr.dev/pkg/xos"
)

// New returns a new manager.
func New() *Manager {
	return &Manager{cache: make(map[string]*Data)}
}

// Manager manages the secrets cache for running Encore apps.
type Manager struct {
	group    singleflight.Group
	pollOnce sync.Once

	mu    sync.Mutex
	cache map[string]*Data
}

// Data is a snapshot of an Encore app's development secret values.
type Data struct {
	// Synced is when the values were last synced,
	// or the zero value if no sync has taken place.
	Synced time.Time
	// Values is a key-value map of defined secrets.
	Values map[string]string
}

type LoadResult struct {
	mgr *Manager
	app *apps.Instance

	once    syncutil.Once
	ch      <-chan singleflight.Result
	initial singleflight.Result
}

// Load loads the secrets for appSlug.
// If appSlug is empty, (*LoadResult).Get resolves to empty secret data.
func (mgr *Manager) Load(app *apps.Instance) *LoadResult {
	mgr.pollOnce.Do(mgr.startPolling)

	// Ignore cases when the app isn't linked.
	if app.PlatformID() == "" {
		return &LoadResult{mgr: mgr, app: app}
	}

	ch := mgr.fetch(app.PlatformID(), false)
	return &LoadResult{mgr: mgr, app: app, ch: ch}
}

// Get returns the result of the prefetch.
// It blocks until the initial fetch is ready or until ctx is cancelled.
// For subsequent calls to Get (such as during live reload), it returns any
// more recent data that has been subsequently cached.
func (lr *LoadResult) Get(ctx context.Context, expSet *experiments.Set) (data *Data, err error) {
	defer func() {
		if err == nil {
			// Return a new data object so we don't write the overrides to the cache.
			data, err = applyLocalOverrides(lr.app, data)
		}
	}()

	if lr == nil || lr.app.PlatformID() == "" {
		return &Data{}, nil
	}

	// Fetch the initial result the first time.
	err = lr.once.Do(func() error {
		select {
		case lr.initial = <-lr.ch:
			// The fetch was successful so mark the Once as completed.
			return nil
		case <-ctx.Done():
			// We timed out before the fetch completed.
			return ctx.Err()
		}
	})
	if err != nil {
		return nil, err
	}

	initial, _ := lr.initial.Val.(*Data)
	haveInitial := lr.initial.Err == nil
	cached, haveCache := lr.mgr.loadFromCache(lr.app.PlatformID())

	switch {
	case haveCache && haveInitial:
		// Which is most recent?
		if initial.Synced.After(cached.Synced) {
			return initial, nil
		} else {
			return cached, nil
		}

	case haveCache:
		return cached, nil

	case haveInitial:
		return initial, nil

	default:
		// We have a prefetch error; return it.
		return nil, lr.initial.Err
	}
}

// UpdateKey updates the cached secret key to the given value.
func (mgr *Manager) UpdateKey(appSlug, key, value string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if data, ok := mgr.cache[appSlug]; ok {
		vals := make(map[string]string)
		for k, v := range data.Values {
			vals[k] = v
		}
		vals[key] = value
		mgr.cache[appSlug] = &Data{
			Synced: time.Now(),
			Values: vals,
		}
		if err := mgr.writeToDisk(appSlug, data); err != nil {
			log.Error().Err(err).Msg("failed to write secrets to disk cache")
		}
	}
}

// fetch fetches secrets from the server.
// mu must not be held when running.
func (mgr *Manager) fetch(appSlug string, poll bool) <-chan singleflight.Result {
	return mgr.group.DoChan(appSlug, func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		secrets, err := platform.GetLocalSecretValues(ctx, appSlug, poll)
		if err != nil {
			return nil, fmt.Errorf("fetch secrets for %s: %v", appSlug, err)
		}
		data := &Data{
			Synced: time.Now(),
			Values: secrets,
		}

		// Update our caches
		mgr.mu.Lock()
		mgr.cache[appSlug] = data
		mgr.mu.Unlock()
		if err := mgr.writeToDisk(appSlug, data); err != nil {
			log.Error().Err(err).Msg("failed to write secrets to disk cache")
		}

		return data, nil
	})
}

func (mgr *Manager) loadFromCache(appSlug string) (*Data, bool) {
	// Do we have the secrets in our cache?
	mgr.mu.Lock()
	data, ok := mgr.cache[appSlug]
	mgr.mu.Unlock()
	if ok {
		return data, true
	}

	// Do we have them on disk?
	if data, err := mgr.readFromDisk(appSlug); err == nil {
		mgr.mu.Lock()
		mgr.cache[appSlug] = data
		mgr.mu.Unlock()
		return data, true
	}
	return nil, false
}

// startPolling begins polling for secret updates every 5 minutes for the apps
// that have been run.
func (mgr *Manager) startPolling() {
	go func() {
		for range time.Tick(5 * time.Minute) {
			var slugs []string
			mgr.mu.Lock()
			for s := range mgr.cache {
				slugs = append(slugs, s)
			}
			mgr.mu.Unlock()

			for _, s := range slugs {
				res := <-mgr.fetch(s, true)
				if res.Err != nil {
					log.Error().Err(res.Err).Str("app_id", s).Msg("failed to sync secrets")
				} else {
					log.Info().Str("app_id", s).Msg("successfully synced app secrets")
				}
			}
		}
	}()
}

// writeToDisk serializes the secret data and writes it to disk
// readable only for the current user.
func (mgr *Manager) writeToDisk(appSlug string, data *Data) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("write secrets %s: %v", appSlug, err)
		}
	}()

	path, err := mgr.secretsPath(appSlug)
	if err != nil {
		return err
	}

	// Create all parent dirs and then chmod the secrets dir to be only user-readable
	secretsDir := filepath.Dir(path)
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		return err
	} else if err := os.Chmod(secretsDir, 0700); err != nil {
		return err
	}

	out, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return xos.WriteFile(path, out, 0600)
}

// readFromDisk reads the cached secrets from disk.
func (mgr *Manager) readFromDisk(appSlug string) (data *Data, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("read secrets %s: %v", appSlug, err)
		}
	}()

	path, err := mgr.secretsPath(appSlug)
	if err != nil {
		return nil, err
	}
	fdata, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = new(Data)
	err = json.Unmarshal(fdata, data)
	return data, err
}

// secretsPath returns the file path to where the given app's secrets are stored on disk.
func (mgr *Manager) secretsPath(appSlug string) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "encore", "secrets", appSlug+".json"), nil
}

// applyLocalOverrides parses the local secrets override file, if any,
// and returns a new Data object with the overrides applied.
//
// If there are no overrides src is returned directly.
// The original src data object is never modified.
func applyLocalOverrides(app *apps.Instance, src *Data) (*Data, error) {
	const name = ".secrets.local.cue"
	data, err := os.ReadFile(filepath.Join(app.Root(), name))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return src, nil
		}
		return nil, err
	}

	updated := &Data{
		Synced: src.Synced,
		Values: make(map[string]string, len(src.Values)),
	}
	for k, v := range src.Values {
		updated.Values[k] = v
	}

	ctx := cuecontext.New()
	loadCfg := &load.Config{
		Stdin: bytes.NewReader(data),
	}

	inst := load.Instances([]string{"-"}, loadCfg)[0]
	if inst.Err != nil {
		return nil, fmt.Errorf("parse local secrets: %v", inst.Err)
	}
	secrets := ctx.BuildInstance(inst)
	if err := secrets.Err(); err != nil {
		return nil, fmt.Errorf("parse local secrets: %v", err)
	}

	it, err := secrets.Fields(cue.Hidden(false), cue.Concrete(true))
	if err != nil {
		return nil, fmt.Errorf("parse local secrets: %v", err)
	}
	for it.Next() {
		key := it.Selector().String()
		val, err := it.Value().String()
		if err != nil {
			return nil, fmt.Errorf("parse local secrets: secret key %s is not a string", key)
		}
		updated.Values[key] = val
	}
	return updated, nil
}
