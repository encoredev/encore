package secrets

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"strings"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/cfgutil"
)

type Manager struct {
	cfg     *config.Runtime
	secrets map[string]string
}

func NewManager(cfg *config.Runtime, infraCfgEnv, appSecretsEnv string) *Manager {
	secrets := parse(appSecretsEnv)
	if infraCfgEnv != "" {
		cfg, err := config.LoadInfraConfig(infraCfgEnv)
		if err != nil {
			log.Fatalln("encore: could not read infra config", err)
		}
		maps.Copy(secrets, cfg.Secrets.GetSecrets())
	}
	return &Manager{cfg: cfg, secrets: secrets}
}

// Load loads a secret.
func (mgr *Manager) Load(key string, inService string) string {
	if val, ok := mgr.secrets[key]; ok {
		return val
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
