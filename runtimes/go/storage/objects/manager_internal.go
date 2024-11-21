package objects

import (
	"context"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/shutdown"
	"encore.dev/appruntime/shared/testsupport"
)

type Manager struct {
	ctx        context.Context
	cancelCtx  func()
	static     *config.Static
	runtime    *config.Runtime
	rt         *reqtrack.RequestTracker
	ts         *testsupport.Manager
	rootLogger zerolog.Logger
	providers  []provider
}

func NewManager(static *config.Static, runtime *config.Runtime, rt *reqtrack.RequestTracker,
	ts *testsupport.Manager, rootLogger zerolog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &Manager{
		ctx:        ctx,
		cancelCtx:  cancel,
		static:     static,
		runtime:    runtime,
		rt:         rt,
		ts:         ts,
		rootLogger: rootLogger,
	}

	for _, p := range providerRegistry {
		mgr.providers = append(mgr.providers, p(mgr.ctx, mgr.runtime))
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
