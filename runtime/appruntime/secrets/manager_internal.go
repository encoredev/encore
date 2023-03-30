package secrets

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"encore.dev/appruntime/config"
)

type Manager struct {
	cfg *config.Config

	// track missing secrets for local development
	missing    []string
	logMissing sync.Once
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

// Load loads a secret.
func (mgr *Manager) Load(key string) string {
	if val, ok := mgr.cfg.Secrets[key]; ok {
		return val
	}

	// For anything but local development, a missing secret is a fatal error.
	if mgr.cfg.Runtime.EnvCloud != "local" {
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
