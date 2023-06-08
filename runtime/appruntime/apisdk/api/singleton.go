//go:build encore_app

package api

import (
	"github.com/benbjohnson/clock"

	encore "encore.dev"
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/health"
	"encore.dev/appruntime/shared/jsonapi"
	"encore.dev/appruntime/shared/logging"
	"encore.dev/appruntime/shared/platform"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/metrics"
	"encore.dev/pubsub"
)

var Singleton = NewServer(
	appconf.Static, appconf.Runtime, reqtrack.Singleton, platform.Singleton,
	encore.Singleton, pubsub.Singleton, logging.RootLogger, metrics.Singleton,
	health.Singleton,
	jsonapi.Default, clock.New(),
)
