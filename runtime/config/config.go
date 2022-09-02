// Package config provides a simple way to access configuration values for a
// service using the Load function.
//
// By default configuration is pulled at build time from CUE files in each service directory
//
// For more information about configuration see https://encore.dev/docs/develop/config.
package config

// Load returns the fully loaded configuration for this service.
//
// Note: This function can only be called from within services.
func Load[T any]() T {
	// Initialise default value
	var rtnValue T

	// FIXME: Initialise the value

	// Return the value
	return rtnValue
}
