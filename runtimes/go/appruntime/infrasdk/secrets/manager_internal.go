package secrets

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"strings"
	"sync"
	"time"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/config/infra"
	"encore.dev/appruntime/shared/cfgutil"
)

// remoteSecretsProvider is implemented by cloud-specific secret backends (e.g. Azure Key Vault).
type remoteSecretsProvider interface {
	FetchSecret(ctx context.Context, name string) (string, error)
}

// newAzureKVProvider is set by azure_keyvault.go when built with Azure support.
// If nil, the Azure Key Vault provider is unavailable.
var newAzureKVProvider func(cfg *infra.AzureKeyVaultSecretsProvider) (remoteSecretsProvider, error)

type Manager struct {
	cfg         *config.Runtime
	secrets     map[string]string
	remote      remoteSecretsProvider
	remoteCache sync.Map // map[string]string — caches values already fetched from remote
}

func NewManager(cfg *config.Runtime, infraCfgEnv, appSecretsEnv string) *Manager {
	secrets := parse(appSecretsEnv)
	var remote remoteSecretsProvider
	if infraCfgEnv != "" {
		infraCfg, err := config.LoadInfraConfig(infraCfgEnv)
		if err != nil {
			log.Fatalln("encore: could not read infra config", err)
		}
		maps.Copy(secrets, infraCfg.Secrets.GetSecrets())

		// Wire up the remote secrets provider if one is configured.
		if p := infraCfg.SecretsProvider; p != nil {
			if kv := p.AzureKeyVault; kv != nil {
				if newAzureKVProvider == nil {
					log.Fatalln("encore: Azure Key Vault secrets provider is configured but Azure support was not compiled in (built with encore_no_azure?)")
				}
				remote, err = newAzureKVProvider(kv)
				if err != nil {
					log.Fatalln("encore: could not initialize Azure Key Vault secrets provider:", err)
				}
			}
		}
	}
	return &Manager{cfg: cfg, secrets: secrets, remote: remote}
}

// Load loads a secret.
func (mgr *Manager) Load(key string, inService string) string {
	if val, ok := mgr.secrets[key]; ok {
		return val
	}

	// Try the remote provider (e.g. Azure Key Vault) with a local in-memory cache.
	if mgr.remote != nil {
		if cached, ok := mgr.remoteCache.Load(key); ok {
			return cached.(string)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if val, err := mgr.remote.FetchSecret(ctx, key); err == nil {
			mgr.remoteCache.Store(key, val)
			return val
		} else {
			fmt.Fprintf(os.Stderr, "encore: error fetching secret %q from remote provider: %v\n", key, err)
		}
	}

	// For anything but local development or a gateway, a missing secret is a fatal error.
	if mgr.cfg.EnvCloud != "local" && cfgutil.IsHostedService(mgr.cfg, inService) {
		fmt.Fprintln(os.Stderr, "encore: could not find secret", key)
		os.Exit(2)
	}

	return ""
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
