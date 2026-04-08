package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"encore.dev/internal/platformauth"
)

func (s *Server) createServiceHandlerAdapter(h Handler) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		// Delete the header so it can't be accessed.
		req.Header.Del("X-Encore-Auth")

		params := toUnnamedParams(ps)

		// Extract metadata from the request.
		meta := CallMetaFromContext(req.Context())

		// Always send the trace id back.
		traceIDStr := meta.TraceID.String()
		w.Header().Set("X-Encore-Trace-ID", traceIDStr)

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

		s.processRequest(h, s.NewIncomingContext(w, req, params, meta))
	}
}

func (s *Server) processRequest(h Handler, c IncomingContext) {
	c.server.beginOperation()
	defer c.server.finishOperation()

	// Pre-compute the endpoint's trace sampling decision and store it in
	// callMeta.TraceSampled. The auth handler uses this (via ParentSampled)
	// instead of making its own sampling decision, which would use the auth
	// handler's endpoint name and typically fall through to the default rate.
	c.callMeta.TraceSampled = s.endpointTraceSampled(h, c)

	info, proceed := s.runAuthHandler(h, c)
	if proceed {
		c.auth = info
		h.Handle(c)
	}
}

// endpointTraceSampled computes the trace sampling decision for the
// given endpoint handler, using the same logic as beginRequest.
func (s *Server) endpointTraceSampled(h Handler, c IncomingContext) bool {
	return s.shouldTrace(
		h.ServiceName(), h.EndpointName(),
		c.req.Header,
		platformauth.IsEncorePlatformRequest(c.req.Context()),
		c.callMeta.ParentSpanID.IsZero(),
		c.callMeta.TraceSampled,
	)
}
