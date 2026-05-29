package secrets

import (
	"encoding/json"

	"encore.dev/types/option"
)

// providersConfig is the JSON shape of the ENCORE_SECRET_PROVIDERS env var.
type providersConfig struct {
	Providers map[string]providerEntry `json:"providers,omitempty"`
}

type providerEntry struct {
	Type   string              `json:"type"`
	Config json.RawMessage     `json:"config,omitempty"`
	Refs   map[string]refEntry `json:"refs,omitempty"`
}

type refEntry struct {
	ID      option.Option[string] `json:"id"`
	Version option.Option[string] `json:"version,omitempty"`
}
