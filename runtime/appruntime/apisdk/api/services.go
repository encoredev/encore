package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (s *Server) createServiceHandlerAdapter(h Handler) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
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

	info, proceed := s.runAuthHandler(h, c)
	if proceed {
		c.auth = info
		h.Handle(c)
	}
}
