// Package secret fetches and caches development secrets for Encore apps.
package secret

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/internal/platform"
	"encr.dev/internal/experiments"
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

// Get gets the secrets for the given app.
func (f *Manager) Get(ctx context.Context, app *apps.Instance, expSet *experiments.Set) (data *Data, err error) {
	f.pollOnce.Do(f.startPolling)

	defer func() {
		if err == nil && experiments.LocalSecretsOverride.Enabled(expSet) {
			// Return a new data object so we don't write the overrides to the cache.
			data, err = f.applyLocalOverrides(app, data)
		}
	}()

	appSlug := app.PlatformID()
	if appSlug == "" {
		return &Data{}, nil
	}

	// Do we have the secrets in our cache?
	f.mu.Lock()
	data, ok := f.cache[appSlug]
	f.mu.Unlock()
	if ok {
		return data, nil
	}

	// Do we have them on disk?
	if data, err := f.readFromDisk(appSlug); err == nil {
		f.mu.Lock()
		f.cache[appSlug] = data
		f.mu.Unlock()
		return data, nil
	}

	return f.fetch(appSlug, false)
}

// UpdateKey updates the cached secret key to the given value.
func (f *Manager) UpdateKey(appSlug, key, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if data, ok := f.cache[appSlug]; ok {
		vals := make(map[string]string)
		for k, v := range data.Values {
			vals[k] = v
		}
		vals[key] = value
		f.cache[appSlug] = &Data{
			Synced: time.Now(),
			Values: vals,
		}
		if err := f.writeToDisk(appSlug, data); err != nil {
			log.Error().Err(err).Msg("failed to write secrets to disk cache")
		}
	}
}

// Prefetch fires off a background task to prefetch secrets for appSlug.
func (f *Manager) Prefetch(appSlug string) {
	// Ignore cases when the app isn't linked.
	if appSlug != "" {
		go f.fetch(appSlug, false)
	}
}

// fetch fetches secrets from the server.
// mu must not be held when running.
func (f *Manager) fetch(appSlug string, poll bool) (*Data, error) {
	data, err, _ := f.group.Do(appSlug, func() (interface{}, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		secrets, err := platform.GetAppSecrets(ctx, appSlug, poll, platform.DevelopmentSecrets)
		if err != nil {
			return nil, fmt.Errorf("fetch secrets for %s: %v", appSlug, err)
		}
		data := &Data{
			Synced: time.Now(),
			Values: secrets,
		}

		// Update our caches
		f.mu.Lock()
		f.cache[appSlug] = data
		f.mu.Unlock()
		if err := f.writeToDisk(appSlug, data); err != nil {
			log.Error().Err(err).Msg("failed to write secrets to disk cache")
		}

		return data, nil
	})
	if err != nil {
		return nil, err
	}
	return data.(*Data), nil
}

// startPolling begins polling for secret updates every 5 minutes for the apps
// that have been run.
func (f *Manager) startPolling() {
	go func() {
		for range time.Tick(5 * time.Minute) {
			var slugs []string
			f.mu.Lock()
			for s := range f.cache {
				slugs = append(slugs, s)
			}
			f.mu.Unlock()

			for _, s := range slugs {
				if _, err := f.fetch(s, true); err != nil {
					log.Error().Err(err).Str("app_id", s).Msg("failed to sync secrets")
				} else {
					log.Info().Str("app_id", s).Msg("successfully synced app secrets")
				}
			}
		}
	}()
}

// writeToDisk serializes the secret data and writes it to disk
// readable only for the current user.
func (f *Manager) writeToDisk(appSlug string, data *Data) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("write secrets %s: %v", appSlug, err)
		}
	}()

	path, err := f.secretsPath(appSlug)
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
	return os.WriteFile(path, out, 0600)
}

// readFromDisk reads the cached secrets from disk.
func (f *Manager) readFromDisk(appSlug string) (data *Data, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("read secrets %s: %v", appSlug, err)
		}
	}()

	path, err := f.secretsPath(appSlug)
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
func (f *Manager) secretsPath(appSlug string) (string, error) {
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
func (f *Manager) applyLocalOverrides(app *apps.Instance, src *Data) (*Data, error) {
	const name = ".secrets.local.cue"
	data, err := os.ReadFile(filepath.Join(app.Root(), name))
	if err != nil {
		if os.IsNotExist(err) {
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
