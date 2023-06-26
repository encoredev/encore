//go:build encore_app

// Package config provides a simple way to access configuration values for a
// service using the Load function.
//
// # By default configuration is pulled at build time from CUE files in each service directory
//
// For more information about configuration see https://encore.dev/docs/develop/config.
package config

import (
	"fmt"

	"encore.dev/appruntime/shared/jsonapi"
	"encore.dev/appruntime/shared/reqtrack"
)

//publicapigen:drop
var Singleton = NewManager(reqtrack.Singleton, jsonapi.Default)

// Load returns the fully loaded configuration for this service.
//
// The configuration is loaded from the CUE files in the service directory and
// will be validated by Encore at compile time, which ensures this function will
// return a valid configuration at runtime.
//
// Encore will generate a `encore.gen.cue` file in the service directory which
// will contain generated CUE matching the configuration type T.
//
// Note: This function can only be called from within services and cannot be
// referenced from other services.
func Load[T any](__serviceName string, __unmarshaler Unmarshaler[T]) T {
	// Get the computed cfg
	cfgBytes, found, err := Singleton.getComputedCUE(__serviceName)
	if err != nil {
		// If the config is not found, return a zero value
		if !found {
			var zero T
			return zero
		}

		panic(err.Error())
	}

	// Create an iterator for the JSON config
	itr := Singleton.json.BorrowIterator(cfgBytes)
	defer Singleton.json.ReturnIterator(itr)
	if itr.Error != nil {
		panic(fmt.Sprintf("failed to unmarshal config for service %s: %v", __serviceName, itr.Error))
	}

	// Now unmarshal the root object
	return __unmarshaler(itr, nil)
}
