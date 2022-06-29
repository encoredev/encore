// Package test provides a number of functions and tools for writing fully integrated test suites for Encore
// applications.
package test

import (
	"encore.dev/runtime/config"
)

func init() {
	if !config.Cfg.Static.Testing {
		panic("You can only import the test package from within a test. None of this packages functionality will work outside of a call to `encore test`")
	}
}
