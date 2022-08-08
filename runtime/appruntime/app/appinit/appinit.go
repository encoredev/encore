//go:build encore_app

package appinit

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"

	// Note: We should not have any dependencies on any of
	// the "encore.dev" packages, since we want to use this
	// package to force initialisation of the Encore runtime to always
	// occur before any user code can execute. Thus almost all packages inside
	// encore.dev will import this package - and that would cause a circular dependency.

	// This needs to be imported to allow JSON Iterator and it's dependancies to have
	// there package level structs loaded, otherwise we get a nil pointer panic.
	// This is due to the fact that the loadAppInstance() call will initialise
	// the JSON iterator for the app, but it's package level variables
	// wouldn't otherwise be setup yet.
	_ "github.com/json-iterator/go"
)

// AppMain is the entrypoint to the Encore Application.
func AppMain() {
	if err := singleton.Run(); err != nil {
		singleton.RootLogger().Fatal().Err(err).Msg("could not run")
	}
}

type AppInstance interface {
	Run() error
	RootLogger() *zerolog.Logger
	GetSecret(key string) (string, bool)
}

// singleton is the instance of the Encore app.
var singleton AppInstance

// loadAppInstance is provided by the "appruntime/app"
// and linked here using go:linkname. This indirection is
// done to avoid a dependency on the "appruntime/app" package which
// would create an circular dependency on the whole runtime.
func loadAppInstance() AppInstance

func init() {
	singleton = loadAppInstance()
}

// LoadSecret loads the secret with the given key.
// If it is not defined it logs a fatal error and exits the process.
func LoadSecret(key string) string {
	if val, ok := singleton.GetSecret(key); ok {
		return val
	}

	fmt.Fprintln(os.Stderr, "encore: could not find secret", key)
	os.Exit(2)
	panic("unreachable")
}
