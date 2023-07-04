package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/shared/cloudtrace"
)

// IsHostedService returns true if the given service is hosted by this instance
// of the Encore application, false otherwise.
func (s *Server) IsHostedService(serviceName string) bool {
	// No runtime configured services or gateways means all services are running here
	if len(s.runtime.HostedServices) == 0 && len(s.runtime.Gateways) == 0 {
		return true
	}

	for _, service := range s.runtime.HostedServices {
		if service == serviceName {
			return true
		}
	}

	return false
}

func (s *Server) createServiceHandlerAdapter(h Handler) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		params := toUnnamedParams(ps)

		// Extract metadata from the request.
		meta := req.Context().Value(metaContextKeyAuthInfo).(CallMeta)

		// Extract any cloud generated Trace identifiers from the request.
		// and use them if we don't have any trace information in the metadata already
		cloudGeneratedTraceIDs := cloudtrace.ExtractCloudTraceIDs(s.rootLogger, req)
		if meta.TraceID.IsZero() {
			meta.TraceID = cloudGeneratedTraceIDs.TraceID
		}

		// SpanID will be zero already, so if our Cloud generated one for us, we should
		// use it as the SpanID for this request
		meta.SpanID = cloudGeneratedTraceIDs.SpanID

		// If we still don't have a trace id, generate one.
		if meta.TraceID.IsZero() {
			meta.TraceID, _ = model.GenTraceID()
			meta.ParentSpanID = model.SpanID{} // no parent span if we have no trace id
		}
		traceIDStr := meta.TraceID.String()

		// Echo the X-Request-ID back to the caller if present,
		// otherwise send back the trace id.
		reqID := req.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = traceIDStr
		} else if len(reqID) > 64 {
			// Don't allow arbitrarily long request IDs.
			s.rootLogger.Warn().Int("length", len(reqID)).Msg("X-Request-ID was too long and is being truncated to 64 characters")
			reqID = reqID[:64]
		}
		w.Header().Set("X-Request-ID", reqID)

		// Read the correlation ID from the request.
		if meta.CorrelationID != "" {
			w.Header().Set("X-Correlation-ID", meta.CorrelationID)
		}

		// Always send the trace id back.
		w.Header().Set("X-Encore-Trace-ID", traceIDStr)

		s.processRequest(h, s.NewIncomingContext(w, req, params, meta))
	}
}
