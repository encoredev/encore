//go:build !encore_app

package app

func initSingletonsForEncoreApp(a *App) {
	// Outside of an Encore application this does nothing,
	// since the singleton variables are not defined in that case.
}
