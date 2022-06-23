package runtime

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"encore.dev/beta/errs"
	"encore.dev/runtime/config"
)

var pubSubSubscriptions = make(map[string]func(r *http.Request) error)

// RegisterPubSubSubscriptionHandler registers a handler for the given PubSub subscription
//
// This is an internal Encore API and should not be used.
func RegisterPubSubSubscriptionHandler(subscriptionID string, handler func(r *http.Request) error) {
	pubSubSubscriptions[subscriptionID] = handler
}

func registerEncoreRoutes(router *httprouter.Router) {
	router.HandlerFunc(wildcardMethod, "/healthz", handleHealthz)
	router.Handle("POST", "/pubsub/push/:subscription_id", handlePubSubPush)
}

// handleHealthz returns the current health and deployment details of the running Encore application
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	bytes, _ := json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details any    `json:"details"`
	}{
		Code:    "ok",
		Message: "Your Encore app is up and running!",
		Details: struct {
			AppRevision    string `json:"app_revision"`
			EncoreCompiler string `json:"encore_compiler"`
			DeployId       string `json:"deploy_id"`
		}{
			AppRevision:    config.Cfg.Static.AppCommit.AsRevisionString(),
			EncoreCompiler: config.Cfg.Static.EncoreCompiler,
			DeployId:       config.Cfg.Runtime.DeployID,
		},
	})
	_, _ = w.Write(bytes)
}

// handlePubSubPush acts like an internal router from the Encore push route, to a registered handler for the given
// subscription
func handlePubSubPush(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	subscriptionID := ps.ByName("subscription_id")
	if subscriptionID == "" {
		errs.HTTPError(w, errs.B().Code(errs.InvalidArgument).Msg("missing subscription ID").Err())
		return
	}

	handler, found := pubSubSubscriptions[subscriptionID]
	if !found {
		errs.HTTPError(w, errs.B().Code(errs.NotFound).Msg("unknown pubsub subscription").Err())
		return
	}

	errs.HTTPError(w, handler(req))
	return
}
