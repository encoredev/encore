package pubsub

import (
	"context"
	"net/http"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/testsupport"
	"encore.dev/beta/errs"
	"encore.dev/pubsub/internal/types"
	"encore.dev/pubsub/internal/utils"
)

type Manager struct {
	ctxs       *utils.Contexts
	static     *config.Static
	runtime    *config.Runtime
	rt         *reqtrack.RequestTracker
	ts         *testsupport.Manager
	rootLogger zerolog.Logger
	json       jsoniter.API
	providers  []provider

	publishCounter  uint64
	pushHandlers    map[types.SubscriptionID]http.HandlerFunc
	runningFetches  sync.WaitGroup
	runningHandlers sync.WaitGroup
}

func NewManager(static *config.Static, runtime *config.Runtime, rt *reqtrack.RequestTracker,
	ts *testsupport.Manager, rootLogger zerolog.Logger, json jsoniter.API) *Manager {
	mgr := &Manager{
		ctxs:         utils.NewContexts(context.Background()),
		static:       static,
		runtime:      runtime,
		rt:           rt,
		ts:           ts,
		rootLogger:   rootLogger,
		json:         json,
		pushHandlers: make(map[types.SubscriptionID]http.HandlerFunc),
	}

	for _, p := range providerRegistry {
		mgr.providers = append(mgr.providers, p(mgr))
	}

	return mgr
}

// Shutdown stops the manager from fetching new messages and processing them.
func (mgr *Manager) Shutdown(force context.Context) {
	// Stop fetching new events and wait for all running fetches to return
	mgr.ctxs.StopFetchingNewEvents()
	mgr.runningFetches.Wait()

	// Now wait for either all handlers to return or the force context to be cancelled
	waitChan := make(chan struct{})
	go func() {
		mgr.runningHandlers.Wait()
		close(waitChan)
	}()

	select {
	case <-force.Done():
	case <-waitChan:
	}

	// Then close all connections to the PubSub providers
	mgr.ctxs.CloseConnections()
}

// StopHandlers cancels the context used for running handlers and waits
// for all running handlers to return.
func (mgr *Manager) StopHandlers() {
	mgr.ctxs.CancelHandler()
	mgr.runningHandlers.Wait()
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
