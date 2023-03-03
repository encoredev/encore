//go:build encore_app

package secrets

//publicapigen:drop
var Singleton *Manager

func Load(key string) string {
	return Singleton.Load(key)
}
