package userconfig

import (
	"os"
	"path/filepath"
	"strings"

	"encr.dev/pkg/xos"
	"github.com/cockroachdb/errors"
	"github.com/pelletier/go-toml"
)

func SetForApp(appRoot, key, value string) error {
	if _, err := os.Stat(appRoot); err != nil {
		return errors.Wrap(err, "app root directory does not exist")
	}
	dst := appFilePath(appRoot)
	return updateConfig(dst, key, value)
}

func SetGlobal(key, value string) error {
	if len(userPaths) == 0 {
		return errors.New("no global config file location found")
	}

	// Find the last path in the list that exists.
	for i := len(userPaths) - 1; i >= 0; i-- {
		if _, err := os.Stat(userPaths[i]); err == nil {
			return updateConfig(userPaths[i], key, value)
		}
	}

	// Otherwise fall back to the lowest-priority entry.
	dst := userPaths[0]
	return updateConfig(dst, key, value)
}

func updateConfig(dstPath, key, value string) error {
	desc, ok := descs[key]
	if !ok {
		return errors.Errorf("unknown key: %q", key)
	}
	val, err := desc.Type.ParseAndValidate(value)
	if err != nil {
		return err
	}

	// Read the existing config.
	// If it doesn't exist it's initialized to an emty config.
	var conf *toml.Tree
	{
		data, err := os.ReadFile(dstPath)
		if err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err, "failed to read existing config")
		}
		if data != nil {
			conf, err = toml.LoadBytes(data)
		} else {
			conf, err = toml.TreeFromMap(map[string]any{})
		}
		if err != nil {
			return errors.Wrap(err, "failed to parse existing config")
		}
	}

	keys := strings.Split(key, ".")
	conf.SetPath(keys, val)

	// Write the config back out.
	data, err := conf.Marshal()
	if err != nil {
		return errors.Wrap(err, "failed to marshal config")
	}

	if err := validateConfig(data); err != nil {
		return errors.Wrap(err, "resulting config is invalid")
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return errors.Wrap(err, "failed to create config file")
	}
	if err := xos.WriteFile(dstPath, data, 0644); err != nil {
		return errors.Wrap(err, "failed to write config file")
	}
	return nil
}
