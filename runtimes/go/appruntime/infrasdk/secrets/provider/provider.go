// Package provider defines the contract between the secrets Manager and
// external secret-resolution backends (GCP Secret Manager, Vault, etc.).
//
// It lives in its own package so that backend implementations can depend on
// it without creating an import cycle with the parent secrets package.
package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"encore.dev/types/option"
)

// Provider resolves a single secret from an external backend.
type Provider interface {
	Load(ctx context.Context, ref Ref) (string, error)
}

// Ref identifies a single secret within a provider.
type Ref struct {
	ID      string
	Version option.Option[string]
}

// Factory constructs a Provider from its type-specific JSON configuration.
type Factory func(rawConfig json.RawMessage) (Provider, error)

var registry = map[string]Factory{}

// Register makes a provider type available to the secrets Manager. Backends
// typically call this from an init function.
func Register(typeName string, f Factory) {
	if _, exists := registry[typeName]; exists {
		panic(fmt.Sprintf("secrets: provider type %q already registered", typeName))
	}
	registry[typeName] = f
}

// Lookup returns the factory registered for the given type name.
func Lookup(typeName string) (Factory, bool) {
	f, ok := registry[typeName]
	return f, ok
}
