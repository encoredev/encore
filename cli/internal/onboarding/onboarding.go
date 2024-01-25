package onboarding

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"encr.dev/pkg/xos"
)

type Event struct {
	time.Time
}

type State struct {
	FirstRun   Event `json:"first_run"`
	DeployHint Event `json:"deploy_hint"`
}

func (e *Event) IsSet() bool {
	return !e.IsZero()
}

func (e *Event) Set() bool {
	if !e.IsSet() {
		e.Time = time.Now()
		return true
	}
	return false
}

func Load() (*State, error) {
	cfg := &State{}
	path, err := configPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

func (cfg *State) Write() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	} else if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return xos.WriteFile(path, data, 0644)
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "encore", "onboarding.json"), nil
}
