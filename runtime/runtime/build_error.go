//go:build !encore_internal

package runtime

// this is a compile error which we've introduced on purpose to prevent accidental inclusion of the encore.dev/runtime package
// outside of Encore applications.
var encoreRuntimeIncludedByAccident = true
var encoreRuntimeIncludedByAccident = true
