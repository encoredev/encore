//go:build encore_app

package app

import (
	encore "encore.dev"
	"encore.dev/appruntime/testsupport"
	"encore.dev/beta/auth"
	"encore.dev/pubsub"
	"encore.dev/rlog"
	"encore.dev/storage/sqldb"
)

func initSingletonsForEncoreApp(a *App) {
	testsupport.Singleton = a.ts
	encore.Singleton = a.encore
	auth.Singleton = a.auth
	rlog.Singleton = a.rlog
	sqldb.Singleton = a.sqldb
	pubsub.Singleton = a.pubsub
}
