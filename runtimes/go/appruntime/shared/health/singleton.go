//go:build encore_app

package health

// Singleton is the singleton instance of the health check registry
// for a running Encore application.
var Singleton = NewCheckRegistry()
