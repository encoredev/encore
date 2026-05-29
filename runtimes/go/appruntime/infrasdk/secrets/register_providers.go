//go:build encore_app

package secrets

// Blank-imported provider packages register themselves via their init() with
// the provider registry. Add new providers (e.g. Vault) here.
import (
	_ "encore.dev/appruntime/infrasdk/secrets/gcpsm"
)
