package secrets

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/infrasdk/secrets/provider"
	"encore.dev/appruntime/shared/cfgutil"
	"encore.dev/types/option"
)

type Manager struct {
	cfg     *config.Runtime
	secrets map[string]string
	refs    map[string]boundRef

	cache   sync.Map // key -> string
	fetcher singleflight.Group
}

func NewManager(cfg *config.Runtime, infraCfgEnv, appSecretsEnv, providersEnv string) *Manager {
	secrets := parse(appSecretsEnv)
	if infraCfgEnv != "" {
		cfg, err := config.LoadInfraConfig(infraCfgEnv)
		if err != nil {
			log.Fatalln("encore: could not read infra config", err)
		}
		maps.Copy(secrets, cfg.Secrets.GetSecrets())
	}
	return &Manager{
		cfg:     cfg,
		secrets: secrets,
		refs:    parseProviders(providersEnv),
	}
}

// Load loads a secret.
func (mgr *Manager) Load(key string, inService string) string {
	if val, ok := mgr.secrets[key]; ok {
		return val
	}

	if val, ok := mgr.loadFromProvider(key); ok {
		return val
	}

	// For anything but local development or a gateway, a missing secret is a fatal error.
	if mgr.cfg.EnvCloud != "local" && cfgutil.IsHostedService(mgr.cfg, inService) {
		fmt.Fprintln(os.Stderr, "encore: could not find secret", key)
		os.Exit(2)
	}

	return ""
}

// loadFromProvider resolves a secret via the configured external provider.
// It returns (value, true) on success, ("", false) when no ref is configured
// for the key, and exits the process when a configured ref fails to resolve
// in a non-local environment.
func (mgr *Manager) loadFromProvider(key string) (string, bool) {
	ref, ok := mgr.refs[key]
	if !ok {
		return "", false
	}

	if cached, ok := mgr.cache.Load(key); ok {
		return cached.(string), true
	}

	val, err, _ := mgr.fetcher.Do(key, func() (interface{}, error) {
		v, err := ref.provider.Load(context.Background(), provider.Ref{ID: ref.ID.GetOrElse(key), Version: ref.Version})
		if err != nil {
			return "", err
		}
		mgr.cache.Store(key, v)
		return v, nil
	})

	if err != nil {
		if mgr.cfg.EnvCloud != "local" {
			fmt.Fprintf(os.Stderr, "encore: could not resolve secret %q: %v\n", key, err)
			os.Exit(2)
		}
		return "", false
	}
	return val.(string), true
}

// boundRef is a configured secret ref bound to the Provider that resolves it.
type boundRef struct {
	provider provider.Provider
	ID       option.Option[string]
	Version  option.Option[string]
}

func parseProviders(s string) map[string]boundRef {
	if s == "" {
		return nil
	}

	// Allow optional gzip+base64 encoding for parity with ENCORE_APP_SECRETS.
	if rest, isGzipped := strings.CutPrefix(s, "gzip:"); isGzipped {
		b, err := base64.StdEncoding.DecodeString(rest)
		if err != nil {
			if b, err = base64.RawURLEncoding.DecodeString(rest); err != nil {
				log.Fatalln("encore runtime: fatal error: could not decode secret providers config:", err)
			}
		}
		gz, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			log.Fatalln("encore runtime: fatal error: could not unzip secret providers config:", err)
		}
		b, err = io.ReadAll(gz)
		if err != nil {
			log.Fatalln("encore runtime: fatal error: could not read secret providers config:", err)
		}
		s = string(b)
	}

	var cfg providersConfig
	if err := json.Unmarshal([]byte(s), &cfg); err != nil {
		log.Fatalln("encore runtime: fatal error: invalid secret providers config:", err)
	}

	refs := make(map[string]boundRef)
	for name, entry := range cfg.Providers {
		factory, ok := provider.Lookup(entry.Type)
		if !ok {
			log.Fatalf("encore runtime: fatal error: unknown secret provider type %q for provider %q", entry.Type, name)
		}
		p, err := factory(entry.Config)
		if err != nil {
			log.Fatalf("encore runtime: fatal error: could not initialize secret provider %q: %v", name, err)
		}

		for key, ref := range entry.Refs {
			if _, ok := refs[key]; ok {
				log.Fatalf("encore runtime: fatal error: secret %q is defined by multiple providers", key)
			}
			refs[key] = boundRef{
				provider: p,
				ID:       ref.ID,
				Version:  ref.Version,
			}
		}
	}
	return refs
}

// parse parses secrets in "key1=base64(val1),key2=base64(val2)" format into a map.
func parse(s string) map[string]string {
	s, isGzipped := strings.CutPrefix(s, "gzip:")
	if isGzipped {
		var b []byte
		var err error

		if b, err = base64.StdEncoding.DecodeString(s); err != nil {
			b, err = base64.RawURLEncoding.DecodeString(s)
		}
		if err != nil {
			log.Fatalln("encore runtime: fatal error: could not decode app secrets:", err)
		}
		gz, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			log.Fatalln("encore runtime: fatal error: could not unzip app secrets:", err)
		}
		b, err = io.ReadAll(gz)
		if err != nil {
			log.Fatalln("encore runtime: fatal error: could not read app secrets:", err)
		}

		s = string(b)
	}
	m := make(map[string]string)
	if s == "" {
		return m
	}
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			log.Fatalln("encore runtime: fatal error: invalid secret value")
		}
		val, err := base64.RawURLEncoding.DecodeString(kv[1])
		if err != nil {
			log.Fatalln("encore runtime: fatal error: invalid secret value")
		}
		m[kv[0]] = string(val)
	}
	return m
}
