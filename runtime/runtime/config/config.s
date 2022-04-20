// Encore's main package uses //go:linkname to push the loadConfig implementation
// into this package, so we need a .s file so the Go tool does not pass -complete
// to "go tool compile" so the latter does not complain about Go functions
// with no bodies.