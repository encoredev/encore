package objects

import (
	"context"

	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/shutdown"
	"encore.dev/appruntime/shared/testsupport"
	"encore.dev/storage/objects/internal/types"
)

type Manager struct {
	ctx        context.Context
	cancelCtx  func()
	static     *config.Static
	runtime    *config.Runtime
	rt         *reqtrack.RequestTracker
	ts         *testsupport.Manager
	rootLogger zerolog.Logger
	json       jsoniter.API
	providers  []provider
}

func NewManager(static *config.Static, runtime *config.Runtime, rt *reqtrack.RequestTracker,
	ts *testsupport.Manager, rootLogger zerolog.Logger, json jsoniter.API) *Manager {
	mgr := &Manager{
		ctx:        context.Background(),
		static:     static,
		runtime:    runtime,
		rt:         rt,
		ts:         ts,
		rootLogger: rootLogger,
		json:       json,
	}

	for _, p := range providerRegistry {
		mgr.providers = append(mgr.providers, p(mgr))
	}

	return mgr
}

// Shutdown stops the manager from fetching new messages and processing them.
func (mgr *Manager) Shutdown(p *shutdown.Process) error {
	// Once it's time to force-close tasks, cancel the base context.
	go func() {
		<-p.ForceCloseTasks.Done()
		mgr.cancelCtx()
	}()

	return nil
}

type provider interface {
	ProviderName() string
	Matches(providerCfg *config.BucketProvider) bool
	NewBucket(providerCfg *config.BucketProvider, staticCfg BucketConfig, runtimeCfg *config.Bucket) types.BucketImpl
}

var providerRegistry []func(*Manager) provider

func registerProvider(p func(mgr *Manager) provider) {
	providerRegistry = append(providerRegistry, p)
}
