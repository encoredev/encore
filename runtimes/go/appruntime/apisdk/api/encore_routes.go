package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"encore.dev/appruntime/shared/jsonapi"
	"encore.dev/beta/errs"
)

func (s *Server) registerEncoreRoutes() {
	s.encore.HandlerFunc(wildcardMethod, "/healthz", s.handleHealthz)
	s.encore.Handle("POST", "/pubsub/push/:subscription_id", s.handlePubsubPush)
	s.encore.Handle("POST", "/authhandler", s.handleRemoteAuthCall)
}

// handleHealthz returns the current health and deployment details of the running Encore application
func (s *Server) handleHealthz(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	statusStr := "ok"
	statusCode := http.StatusOK

	// Run all health checks
	type checkResult struct {
		Name   string `json:"name"`
		Passed bool   `json:"passed"`
		Error  string `json:"error,omitempty"`
	}
	var checkResults []checkResult
	for _, result := range s.healthMgr.RunAll(req.Context()) {
		errStr := ""
		if result.Err != nil {
			statusStr = "unhealthy"
			statusCode = http.StatusInternalServerError
			errStr = result.Err.Error()
		}

		checkResults = append(checkResults, checkResult{
			Name:   result.Name,
			Passed: result.Err == nil,
			Error:  errStr,
		})
	}

	w.WriteHeader(statusCode)
	bytes, _ := jsonapi.Default.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details any    `json:"details"`
	}{
		Code:    statusStr,
		Message: "Your Encore app is up and running!",
		Details: struct {
			AppRevision        string        `json:"app_revision"`
			EncoreCompiler     string        `json:"encore_compiler"`
			DeployId           string        `json:"deploy_id"`
			Checks             []checkResult `json:"checks"`
			EnabledExperiments []string      `json:"enabled_experiments"`
		}{
			AppRevision:        s.static.AppCommit.AsRevisionString(),
			EncoreCompiler:     s.static.EncoreCompiler,
			DeployId:           s.runtime.DeployID,
			Checks:             checkResults,
			EnabledExperiments: s.experiments.StringList(),
		},
	})
	// nosemgrep
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
	// Is this a gateway and the pubsub subscription isn't hosted here?
	// If so forward the request to the target service instead.
	if remoteSubHandler, ok := s.remotePubSubPush[subscriptionID]; ok {
		if err := remoteSubHandler.ForwardRequest(w, req); err != nil {
			errs.HTTPError(w, err)
		}
		return
	}

	s.pubsubMgr.HandlePubSubPush(w, req, subscriptionID)
}
