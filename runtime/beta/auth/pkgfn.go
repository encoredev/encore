//go:build encore_app

package auth

var Singleton *Manager // injected on app init

func UserID() (UID, bool) {
	return Singleton.UserID()
}

func Data() any {
	return Singleton.Data()
}
