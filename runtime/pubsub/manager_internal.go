package pubsub

import (
	"context"
	"net/http"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/shutdown"
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
func (mgr *Manager) Shutdown(p *shutdown.Process) error {
	// Once it's time to force-close tasks, cancel the base context.
	go func() {
		<-p.ForceCloseTasks.Done()
		mgr.ctxs.CancelHandler()
	}()

	p.Log.Trace().Msg("pubsub: stop fetching new events")

	// Immediately fetching new events.
	mgr.ctxs.StopFetchingNewEvents()
	p.Log.Trace().Msg("pubsub: waiting on running fetches")
	mgr.runningFetches.Wait()

	// Wait for running handlers to finish.
	mgr.runningHandlers.Wait()
	p.MarkOutstandingPubSubMessagesCompleted()

	// Finally, close all connections to the PubSub providers.
	mgr.ctxs.CloseConnections()

	return nil
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
