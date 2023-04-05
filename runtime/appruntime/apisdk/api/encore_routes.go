package api

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"encore.dev/beta/errs"
)

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
			AppRevision:    s.static.AppCommit.AsRevisionString(),
			EncoreCompiler: s.static.EncoreCompiler,
			DeployId:       s.runtime.DeployID,
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
	s.pubsubMgr.HandlePubSubPush(w, req, subscriptionID)
}
