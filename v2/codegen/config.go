package codegen

import (
	"encr.dev/v2/internals/pkginfo"
)

// TestConfig describes common configuration for code generation
// when running tests.
type TestConfig struct {
	// Packages are the packages to generate test code for.
	Packages []*pkginfo.Package

	// EnvsToEmbed are the environment variables to embed inside
	// the test binaries themselves. This is useful when
	// building tests with "go test -c", where the binary is
	// built first and executed later (such as by GoLand).
	EnvsToEmbed []string
}
