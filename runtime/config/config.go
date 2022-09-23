// Package config provides a simple way to access configuration values for a
// service using the Load function.
//
// # By default configuration is pulled at build time from CUE files in each service directory
//
// For more information about configuration see https://encore.dev/docs/develop/config.
package config

import (
	"encoding/base64"
	"fmt"
	"os"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.Config{
	EscapeHTML:             false,
	SortMapKeys:            true,
	ValidateJsonRawMessage: true,
}.Froze()

// Load returns the fully loaded configuration for this service.
//
// Note: This function can only be called from within services.
func Load[T any](__serviceName string, __unmarshaler Unmarshaler[T]) T {
	// Fetch the raw JSON config for this service
	envVar := os.Getenv(envName(__serviceName))
	if envVar == "" {
		panic(fmt.Sprintf("configuration for service `%s` not found, expected it in envriomental variable %s", __serviceName, envName(__serviceName)))
	}
	cfgBytes, err := base64.RawURLEncoding.DecodeString(envVar)
	if err != nil {
		panic(fmt.Sprintf("failed to decode configuration for service `%s`: %v", __serviceName, err))
	}

	// Create an iterator for the JSON config
	itr := json.BorrowIterator(cfgBytes)
	defer json.ReturnIterator(itr)
	if itr.Error != nil {
		panic(fmt.Sprintf("failed to unmarshal config for service %s: %v", __serviceName, itr.Error))
	}

	// Now unmarshal the root object
	return __unmarshaler(itr, nil)
}
