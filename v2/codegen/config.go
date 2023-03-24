package codegen

import (
	"encr.dev/v2/internals/pkginfo"
)

// TestConfig describes common configuration for code generation
// when running tests.
type TestConfig struct {
	// Packages are the packages to generate test code for.
	Packages []*pkginfo.Package
}
