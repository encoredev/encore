package pubsub

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"

	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/testsupport"
	"encore.dev/beta/errs"
	"encore.dev/pubsub/internal/types"
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

	publishCounter uint64
	outstanding    *outstandingMessageTracker
	pushHandlers   map[types.SubscriptionID]http.HandlerFunc
}

func NewManager(static *config.Static, runtime *config.Runtime, rt *reqtrack.RequestTracker,
	ts *testsupport.Manager, rootLogger zerolog.Logger, json jsoniter.API) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &Manager{
		ctx:          ctx,
		cancelCtx:    cancel,
		static:       static,
		runtime:      runtime,
		rt:           rt,
		ts:           ts,
		rootLogger:   rootLogger,
		json:         json,
		outstanding:  newOutstandingMessageTracker(),
		pushHandlers: make(map[types.SubscriptionID]http.HandlerFunc),
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
	NewTopic(providerCfg *config.PubsubProvider, staticCfg TopicConfig, runtimeCfg *config.PubsubTopic) types.TopicImplementation
}

var providerRegistry []func(*Manager) provider

func registerProvider(p func(mgr *Manager) provider) {
	providerRegistry = append(providerRegistry, p)
}

var _ types.PushEndpointRegistry = (*Manager)(nil)

func (mgr *Manager) RegisterPushSubscriptionHandler(id types.SubscriptionID, handler http.HandlerFunc) {
	mgr.pushHandlers[id] = handler
}

// HandlePubSubPush is an HTTP handler that accepts PubSub push HTTP requests
// and routes them to the appropriate push handler.
func (mgr *Manager) HandlePubSubPush(w http.ResponseWriter, req *http.Request, subscriptionID string) {
	handler, found := mgr.pushHandlers[types.SubscriptionID(subscriptionID)]
	if !found {
		err := errs.B().Code(errs.NotFound).Msg("unknown pubsub subscription").Err()
		mgr.rootLogger.Err(err).Str("subscription_id", subscriptionID).Msg("invalid PubSub push request")
		errs.HTTPError(w, err)
		return
	}

	handler(w, req)
}
