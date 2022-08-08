//go:build encore_app

// Package appinit exists to ensure the runtime is initialized before
// user code runs. It can safely be depended on by any package as it
// itself has no dependencies on any other package.
package appinit

func init() {
	doInit()
}

//go:linkname doInit encore.dev/appruntime/app.doInit
func doInit()
