package transport

// These are predefined keys for metadata that we use in Encore.
//
// which allow each transport method know about them and handle them
// in a transport specific way if needed (including renaming them)
const (
	TraceParentKey   = "Traceparent"
	TraceStateKey    = "Tracestate"
	CorrelationIDKey = "Correlation-ID"
)
