//go:build encore_app

package app

import (
	encore "encore.dev"
	"encore.dev/appruntime/api"
	"encore.dev/appruntime/secrets"
	"encore.dev/appruntime/service"
	"encore.dev/appruntime/testsupport"
	"encore.dev/beta/auth"
	"encore.dev/config"
	"encore.dev/et"
	"encore.dev/metrics"
	"encore.dev/pubsub"
	"encore.dev/rlog"
	"encore.dev/storage/cache"
	"encore.dev/storage/sqldb"
)

func initSingletonsForEncoreApp(a *App) {
	api.Singleton = a.api
	service.Singleton = a.service
	testsupport.Singleton = a.ts
	encore.Singleton = a.encore
	secrets.Singleton = a.secrets
	auth.Singleton = a.auth
	rlog.Singleton = a.rlog
	sqldb.Singleton = a.sqldb
	pubsub.Singleton = a.pubsub
	cache.Singleton = a.cache
	config.Singleton = a.config
	et.Singleton = a.et
	metrics.Singleton = a.metricsRegistry
}
