package pubsub

import (
	"context"
	"sync"
	"sync/atomic"

	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/api"
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/reqtrack"
	"encore.dev/appruntime/testsupport"
	"encore.dev/pubsub/internal/types"
)

type Manager struct {
	ctx        context.Context
	cancelCtx  func()
	cfg        *config.Config
	rt         *reqtrack.RequestTracker
	apiSrv     *api.Server
	ts         *testsupport.Manager
	rootLogger zerolog.Logger
	json       jsoniter.API
	providers  []provider

	publishCounter uint64
	outstanding    *outstandingMessageTracker
}

func NewManager(cfg *config.Config, rt *reqtrack.RequestTracker, ts *testsupport.Manager, server *api.Server, rootLogger zerolog.Logger, json jsoniter.API) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &Manager{
		ctx:         ctx,
		cancelCtx:   cancel,
		cfg:         cfg,
		rt:          rt,
		apiSrv:      server,
		ts:          ts,
		rootLogger:  rootLogger,
		json:        json,
		outstanding: newOutstandingMessageTracker(),
	}

	for _, p := range providerRegistry {
		mgr.providers = append(mgr.providers, p(mgr))
	}

	return mgr
}

func (mgr *Manager) Shutdown(force context.Context) {
	mgr.cancelCtx()
	mgr.outstanding.ArmForShutdown()

	select {
	case <-mgr.outstanding.Done():
	case <-force.Done():
	}
}

// outstandingMessageTracker tracks the number of outstanding messages.
// Once Shutdown() has been called, the next time the number of outstanding
// messages reaches zero (or if it's already zero), the Done() channel is closed.
type outstandingMessageTracker struct {
	active int64

	closeOnce sync.Once
	done      chan struct{}

	mu    sync.Mutex
	armed bool
}

func newOutstandingMessageTracker() *outstandingMessageTracker {
	return &outstandingMessageTracker{
		done: make(chan struct{}),
	}
}

func (t *outstandingMessageTracker) Inc() {
	atomic.AddInt64(&t.active, 1)
}

func (t *outstandingMessageTracker) Dec() {
	val := atomic.AddInt64(&t.active, -1)
	if val < 0 {
		panic("outstandingMessageTracker: active < 0")
	}
	if val == 0 {
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.armed {
			t.markDone()
		}
	}
}

func (t *outstandingMessageTracker) ArmForShutdown() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.armed = true

	// If we're already at zero, mark it as done right away.
	if atomic.LoadInt64(&t.active) == 0 {
		t.markDone()
	}
}

// markDone marks the tracker as being done.
func (t *outstandingMessageTracker) markDone() {
	t.closeOnce.Do(func() { close(t.done) })
}

func (t *outstandingMessageTracker) Done() <-chan struct{} {
	return t.done
}

type provider interface {
	ProviderName() string
	Matches(providerCfg *config.PubsubProvider) bool
	NewTopic(providerCfg *config.PubsubProvider, topicCfg *config.PubsubTopic) types.TopicImplementation
}

var providerRegistry []func(*Manager) provider

func registerProvider(p func(mgr *Manager) provider) {
	providerRegistry = append(providerRegistry, p)
}
