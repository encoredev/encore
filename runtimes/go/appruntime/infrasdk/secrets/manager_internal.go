package secrets

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/cfgutil"
)

type Manager struct {
	cfg     *config.Runtime
	secrets map[string]string
}

func NewManager(cfg *config.Runtime, appSecretsEnv string) *Manager {
	return &Manager{cfg: cfg, secrets: parse(appSecretsEnv)}
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
