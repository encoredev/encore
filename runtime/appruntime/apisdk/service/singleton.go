//go:build encore_app

package service

import (
	"encore.dev/appruntime/shared/logging"
	"encore.dev/appruntime/shared/reqtrack"
)

var Singleton = NewManager(reqtrack.Singleton, logging.RootLogger)

func Register(i Initializer) {
	Singleton.RegisterService(i)
}
