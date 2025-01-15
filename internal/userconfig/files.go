package userconfig

import (
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"encr.dev/internal/goldfish"
	"github.com/cockroachdb/errors"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
)

const globalCacheKey = "#global#"

var (
	goldfishMu sync.Mutex
	goldfishes = make(map[string]*Cached)
)

type Cached = goldfish.Cache[*Config]

func ForApp(appRoot string) *Cached {
	appRoot = filepath.Clean(appRoot)
	paths := slices.Clone(userPaths)
	paths = append(paths, appFilePath(appRoot))
	return forCacheKey(appRoot, paths)
}

func Global() *Cached {
	return forCacheKey(globalCacheKey, userPaths)
}

func forCacheKey(key string, paths []string) *Cached {
	goldfishMu.Lock()
	defer goldfishMu.Unlock()

	if c, ok := goldfishes[key]; ok {
		return c
	}

	c := goldfish.New(1*time.Second, func() (*Config, error) {
		return newInstance(paths...)
	})
	goldfishes[key] = c
	return c
}

func appFilePath(appRoot string) string {
	return filepath.Join(appRoot, ".encore", "config")
}

var userPaths []string = func() []string {
	var paths []string

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome != "" {
		paths = append(paths, filepath.Join(configHome, "encore", "config"))
	}

	if u, err := user.Current(); err == nil {
		if configHome == "" {
			paths = append(paths, filepath.Join(u.HomeDir, ".config", "encore", "config"))
		}
		paths = append(paths, filepath.Join(u.HomeDir, ".encoreconfig"))
	}

	return paths
}()

var tomlParser = toml.Parser()

func newInstance(paths ...string) (*Config, error) {
	k := koanf.New(".")

	for _, path := range paths {
		f := file.Provider(path)
		err := k.Load(f, tomlParser)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, errors.Wrap(err, "unable to parse config file")
		}
	}

	cfg := &Config{}
	err := k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{
		Tag:       "koanf",
		FlatPaths: true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal config")
	}
	return cfg, nil
}

func validateConfig(data []byte) error {
	k := koanf.New(".")
	return k.Load(rawbytes.Provider(data), tomlParser)
}
