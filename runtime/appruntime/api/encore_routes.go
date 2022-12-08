package api

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"encore.dev/beta/errs"
)

// RegisterPubsubSubscriptionHandler registers a handler for the given PubSub subscription
//
// This is an internal Encore API and should not be used.
func (s *Server) RegisterPubsubSubscriptionHandler(subscriptionID string, handler func(r *http.Request) error) {
	s.pubsubSubscriptions[subscriptionID] = handler
}

func (s *Server) registerEncoreRoutes() {
	s.encore.HandlerFunc(wildcardMethod, "/healthz", s.handleHealthz)
	s.encore.Handle("POST", "/pubsub/push/:subscription_id", s.handlePubsubPush)
}

// handleHealthz returns the current health and deployment details of the running Encore application
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
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
			AppRevision:    s.cfg.Static.AppCommit.AsRevisionString(),
			EncoreCompiler: s.cfg.Static.EncoreCompiler,
			DeployId:       s.cfg.Runtime.DeployID,
		},
	})
	_, _ = w.Write(bytes)
}

// handlePubsubPush acts like an internal router from the Encore push route, to a registered handler for the given
// subscription
func (s *Server) handlePubsubPush(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	subscriptionID := ps.ByName("subscription_id")
	if subscriptionID == "" {
		err := errs.B().Code(errs.InvalidArgument).Msg("missing subscription ID").Err()
		s.rt.Logger().Err(err).Str("subscription_id", subscriptionID).Msg("invalid PubSub push request")
		errs.HTTPError(w, err)
		return
	}

	handler, found := s.pubsubSubscriptions[subscriptionID]
	if !found {
		err := errs.B().Code(errs.NotFound).Msg("unknown pubsub subscription").Err()
		s.rt.Logger().Err(err).Str("subscription_id", subscriptionID).Msg("invalid PubSub push request")
		errs.HTTPError(w, err)
		return
	}

	err := handler(req)
	if err != nil {
		s.rt.Logger().Err(err).Str("subscription_id", subscriptionID).Msg("error while handling PubSub push request")
	}
	errs.HTTPError(w, err)
}
