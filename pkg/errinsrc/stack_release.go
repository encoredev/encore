//go:build !dev_build

package errinsrc

// IncludeStackByDefault is whether to include the stack by default on all errors.
// This is exported to allow Encore's CI platform to set this to true for CI/CD builds.
var IncludeStackByDefault = false
