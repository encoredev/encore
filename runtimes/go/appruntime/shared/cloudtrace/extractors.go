package cloudtrace

import (
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/model"
)

type TraceContext struct {
	TraceID model.TraceID // The trace ID we are in (zero if not set yet)
	SpanID  model.SpanID  // The span ID that represents this request (zero if not set by the cloud provider)
}

// parseB3Headers parses the B3 headers from the request and returns the trace context.
func parseB3Headers(logger zerolog.Logger, r *http.Request) *TraceContext {
	traceIDHex := r.Header.Get("X-B3-TraceId")
	spanIDHex := r.Header.Get("X-B3-SpanId") // Note: ParentSpanId is used to denote the parent span ID in the B3 spec

	if traceIDHex == "" || spanIDHex == "" {
		return nil
	}

	// Extract
	traceID, err := hex.DecodeString(traceIDHex)
	if err != nil {
		logger.Warn().Err(err).Str("header", traceIDHex).Msg("Failed to decode X-B3-TraceId header")
		return nil
	}
	spanID, err := hex.DecodeString(spanIDHex)
	if err != nil {
		logger.Warn().Err(err).Str("header", spanIDHex).Msg("Failed to decode X-B3-SpanId header")
		return nil
	}

	// Validate
	if len(traceID) != 16 {
		logger.Warn().Str("header", traceIDHex).Msg("X-B3-TraceId header was not 16 bytes")
		return nil
	}
	if len(spanID) != 8 {
		logger.Warn().Str("header", spanIDHex).Msg("X-B3-SpanId header was not 8 bytes")
		return nil
	}

	// Return
	var traceContext TraceContext
	copy(traceContext.TraceID[:], traceID)
	copy(traceContext.SpanID[:], spanID)
	return &traceContext
}

// parseB3SingleHeader parses the B3 single header from the request and returns the trace context.
func parseB3SingleHeader(logger zerolog.Logger, r *http.Request) *TraceContext {
	b3Header := r.Header.Get("b3")
	parts := strings.Split(b3Header, "-")
	if len(parts) < 2 {
		return nil
	}

	// Extract
	traceID, err := hex.DecodeString(parts[0])
	if err != nil {
		logger.Warn().Err(err).Str("header", b3Header).Msg("Failed to decode b3 header, trace ID was not hex encoded")
		return nil
	}
	spanID, err := hex.DecodeString(parts[1])
	if err != nil {
		logger.Warn().Err(err).Str("header", b3Header).Msg("Failed to decode b3 header, span ID was not hex encoded")
		return nil
	}

	// Validate
	if len(traceID) != 16 {
		logger.Warn().Str("header", b3Header).Msg("Failed to decode b3 header, trace ID was not 16 bytes")
		return nil
	}
	if len(spanID) != 8 {
		logger.Warn().Str("header", b3Header).Msg("Failed to decode b3 header, span ID was not 8 bytes")
		return nil
	}

	// Return
	var traceContext TraceContext
	copy(traceContext.TraceID[:], traceID)
	copy(traceContext.SpanID[:], spanID)
	return &traceContext
}

// parseGCloudTraceContext parses the Google Cloud Trace header from the request and returns the trace context.
func parseGCloudTraceContext(logger zerolog.Logger, r *http.Request) *TraceContext {
	traceHeader := r.Header.Get("X-Cloud-Trace-Context")
	parts := strings.SplitN(traceHeader, "/", 2)
	if len(parts) < 2 {
		return nil
	}

	// Extract
	traceIDHex := parts[0]
	spanIDParts := strings.SplitN(parts[1], ";", 2)
	spanIDDecStr := spanIDParts[0] // Split on semicolon to ignore optional fields (like ;o=1)

	traceID, err := hex.DecodeString(traceIDHex)
	if err != nil {
		logger.Warn().Err(err).Str("header", traceHeader).Msg("Failed to decode X-Cloud-Trace-Context header, trace ID was not hex encoded")
		return nil
	}
	spanIDDec, err := strconv.ParseUint(spanIDDecStr, 10, 64)
	if err != nil {
		logger.Warn().Err(err).Str("header", traceHeader).Msg("Failed to decode X-Cloud-Trace-Context header, span ID was not a decimal number")
		return nil
	}

	// Validate
	if len(traceID) != 16 {
		logger.Warn().Str("header", traceHeader).Msg("Failed to decode X-Cloud-Trace-Context header, trace ID was not 16 bytes")
		return nil
	}

	// Return
	var traceContext TraceContext
	copy(traceContext.TraceID[:], traceID)
	binary.BigEndian.PutUint64(traceContext.SpanID[:], spanIDDec)
	return &traceContext
}

// parseAWSXrayTraceContext returns the Trace ID
func parseAWSXRayTraceContext(logger zerolog.Logger, r *http.Request) *TraceContext {
	traceHeader := r.Header.Get("X-Amzn-Trace-Id")
	parts := strings.Split(traceHeader, ";")
	traceIDHex := ""

	// Extract
	for _, part := range parts {
		keyValue := strings.Split(part, "=")
		if len(keyValue) != 2 {
			continue
		}

		switch keyValue[0] {
		case "Root":
			traceParts := strings.Split(keyValue[1], "-")
			if len(traceParts) != 3 {
				continue
			}
			traceIDHex = traceParts[2]
		}
	}

	if traceIDHex == "" {
		return nil
	}

	traceID, err := hex.DecodeString(traceIDHex)
	if err != nil {
		logger.Warn().Str("header", traceHeader).Msg("Failed to decode X-Amzn-Trace-Id header, trace ID was not hex")
		return nil
	}

	// Validate
	if len(traceID) != 16 {
		logger.Warn().Str("header", traceHeader).Msg("Failed to decode X-Amzn-Trace-Id header, trace ID was not 16 bytes")
		return nil
	}

	// Return
	var traceContext TraceContext
	copy(traceContext.TraceID[:], traceID)
	// AWS does not generate a SpanID for us
	return nil
}

// parseTraceparent returns the standardised trace parent header, which doesn't give us an ID for this span
// but does given us a trace ID
func parseTraceParent(logger zerolog.Logger, r *http.Request) *TraceContext {
	traceHeader := r.Header.Get("traceparent")

	// Extract
	parts := strings.Split(traceHeader, "-")
	if len(parts) < 3 {
		return nil
	}
	traceID, err := hex.DecodeString(parts[1])
	if err != nil {
		logger.Warn().Str("header", traceHeader).Msg("Failed to decode traceparent header, trace ID was not hex")
		return nil
	}

	// Validate
	if len(traceID) != 16 {
		logger.Warn().Str("header", traceHeader).Msg("Failed to decode traceparent header, trace ID was not 16 bytes")
		return nil
	}

	// Return
	var traceContext TraceContext
	copy(traceContext.TraceID[:], traceID)
	// traceparent doesn't tell give us a span ID for this request
	return &traceContext
}

// ExtractCloudTraceIDs extracts the clouds expected Trace ID and Span ID for this request.
//
// This function will never return nil
func ExtractCloudTraceIDs(logger zerolog.Logger, req *http.Request) *TraceContext {
	extractors := []func(zerolog.Logger, *http.Request) *TraceContext{
		parseGCloudTraceContext,  // GCP specific and gives us _our_ span ID
		parseAWSXRayTraceContext, // AWS specific and does not given us a span ID
		parseB3Headers,           // Older standard which does give us a span ID, but is cloud agnostic
		parseB3SingleHeader,      // Different variant of the B3 standard, which gives us a cloud ID
		parseTraceParent,         // We do this last, as ideally we want a span ID which the cloud already knows about for this request
	}

	for _, extractor := range extractors {
		if traceContext := extractor(logger, req); traceContext != nil {
			return traceContext
		}
	}

	return &TraceContext{}
}
