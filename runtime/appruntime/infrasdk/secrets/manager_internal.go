package secrets

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"encore.dev/appruntime/exported/config"
)

type Manager struct {
	cfg     *config.Runtime
	secrets map[string]string

	// track missing secrets for local development
	missing    []string
	logMissing sync.Once
}

func NewManager(cfg *config.Runtime, appSecretsEnv string) *Manager {
	return &Manager{cfg: cfg, secrets: parse(appSecretsEnv)}
}

// Load loads a secret.
func (mgr *Manager) Load(key string) string {
	if val, ok := mgr.secrets[key]; ok {
		return val
	}

	// For anything but local development, a missing secret is a fatal error.
	if mgr.cfg.EnvCloud != "local" {
		fmt.Fprintln(os.Stderr, "encore: could not find secret", key)
		os.Exit(2)
	}

	mgr.missing = append(mgr.missing, key)
	mgr.logMissing.Do(func() {
		// Wait one second before logging all the missing secrets.
		go func() {
			time.Sleep(1 * time.Second)
			fmt.Fprintln(os.Stderr, "\n\033[31mwarning: secrets not defined:", strings.Join(mgr.missing, ", "), "\033[0m")
			fmt.Fprintln(os.Stderr, "\033[2mnote: undefined secrets are left empty for local development only.")
			fmt.Fprint(os.Stderr, "see https://encore.dev/docs/primitives/secrets for more information\033[0m\n\n")
		}()
	})

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
