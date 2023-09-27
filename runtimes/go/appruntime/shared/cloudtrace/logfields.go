package cloudtrace

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

// StructuredLogFields returns a map of structured log fields that should be
// added to the log entry for the given request such that the cloud's native
// tracing system can pick up the log entry and associate it with the trace.
func StructuredLogFields(req *http.Request) map[string]string {
	// No request, no fields
	if req == nil {
		return nil
	}

	additionalLogFields := make(map[string]string)

	// On GCP if we add their trace context to a specific log field
	// then it will automatically be picked up by Stackdriver and
	// associated with the trace.
	if gcpTraceContext := req.Header.Get("X-Cloud-Trace-Context"); gcpTraceContext != "" {
		gcpProjectID := GcpProjectID()
		if gcpProjectID != "" {
			ctx := parseGCloudTraceContext(log.Logger, req)

			// Add the trace and span fields to the log entry
			if !ctx.TraceID.IsZero() {
				additionalLogFields["logging.googleapis.com/trace"] = fmt.Sprintf("projects/%s/traces/%x", gcpProjectID, ctx.TraceID[:])

				if !ctx.SpanID.IsZero() {
					// Google specifies we should use hex encoding for the span ID on these logs so the Trace Explorer can pick them up.
					// (even though the span ID is a uint64 when passed in the header & Google's own access logging using the integer version
					// the UI expects a hex string)
					additionalLogFields["logging.googleapis.com/spanId"] = fmt.Sprintf("%x", ctx.SpanID[:])
				}
			}
		}
	}

	return additionalLogFields
}
