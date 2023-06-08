package api

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"

	"encore.dev/appruntime/apisdk/api/svcauth"
	"encore.dev/appruntime/apisdk/api/transport"
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/model"
	"encore.dev/beta/errs"
)

var (
	// authJSON can not be pretty printed, as new lines are not allowed in HTTP headers
	authJSON = jsoniter.Config{
		IndentionStep:          0,
		ValidateJsonRawMessage: true,
		TagKey:                 "auth-json",
	}.Froze()
)

// CallMeta is metadata for an RPC call
type CallMeta struct {
	// Untrusted fields
	// (i.e. we allow these fields to be passed in from anywhere, including external requests)
	TraceID       model.TraceID      // The trace ID of the calling request (zero if not tracing)
	ParentSpanID  model.SpanID       // The span ID of the calling request (zero if there's no parent)
	ParentEventID model.TraceEventID // The event ID which started the RPC call (zero if there's no parent)
	CorrelationID string             // The correlation ID of the calling request

	// Internal meta data which gets populated by Encore on service to service calls
	//
	// If set, the values can be trusted as they would have been authenticated to be correct
	Internal *InternalCallMeta
}

// InternalCallMeta is metadata for an RPC call which is being made
// between two Encore services within the same application.
type InternalCallMeta struct {
	SendingService string // The name of the service which is making the call TODO(domblack): maybe make this struct?
	AuthUID        string // The UID of the authenticated user
	AuthData       any    // The data of the authenticated user
}

// addInternalCallMeta adds internal metadata to the external request
// we're about to make.
//
// It does this in a transport agnostic way, allowing us to add metadata
// to any transport request supported by Encore.
func (s *Server) metaFromAPICall(call *model.APICall) (meta CallMeta, err error) {
	// Check the auth data before we marshal
	// this is because auth.WithContext can set the auth data to any type
	// and we want to ensure it's the expected type before we marshal it
	if err := CheckAuthData(call.UserID, call.AuthData); err != nil {
		return meta, err
	}

	if call != nil && call.Source != nil {
		meta.TraceID = call.Source.TraceID
		meta.ParentSpanID = call.Source.SpanID
		meta.ParentEventID = call.StartEventID
		meta.CorrelationID = call.Source.ExtCorrelationID

		meta.Internal = &InternalCallMeta{
			SendingService: s.static.BundledServices[call.Source.SvcNum-1],
			AuthUID:        string(call.UserID),
			AuthData:       call.AuthData,
		}
	} else {
		// If there's no call request, we're probably in the middle of system startup
		// so we'll just use the first bundled service as the sending service
		meta.Internal = &InternalCallMeta{
			SendingService: s.static.BundledServices[0],
			AuthUID:        string(call.UserID),
			AuthData:       call.AuthData,
		}
	}

	return meta, nil
}

// AddToRequest adds the metadata to the given request
func (meta CallMeta) AddToRequest(server *Server, targetService config.Service, req transport.Transport) error {
	// Future proofing: if we ever create a breaking change to the transport meta
	// we can use this version number to indicate which version of the meta we're using
	req.SetMeta("Version", "1")

	// If we're tracing, pass the trace ID, span ID and event ID to the downstream service
	if !meta.TraceID.IsZero() {
		// Encode Encore's trace ID and span ID as the traceparent header
		req.SetMeta(transport.TraceParentKey, fmt.Sprintf("00-%x-%x-01", meta.TraceID[:], meta.ParentSpanID[:]))

		// Because Encore does not count an RPC call as a span, but rather a set of events within a span
		// we also need to pass the event ID which started the RPC call in the tracestate header
		eventID := strconv.FormatUint(uint64(meta.ParentEventID), 36)
		req.SetMeta(transport.TraceStateKey, fmt.Sprintf("%s=%s", eventTraceStateKey, eventID))
	}

	// Pass the correlation ID to the downstream service.
	// However, we do _not_ pass the X-Request-ID down, as it is not meant to be propagated through request chains
	if meta.CorrelationID != "" {
		req.SetMeta(transport.CorrelationIDKey, meta.CorrelationID)
	}

	// If we're making an internal call, add the internal metadata to the request
	if meta.Internal != nil {
		// Add a marker to the request to indicate that this is an internal call
		req.SetMeta("Internal-Call", meta.Internal.SendingService)

		// Add the auth data
		if meta.Internal.AuthUID != "" {
			req.SetMeta("UserID", meta.Internal.AuthUID)

			if meta.Internal.AuthData != nil {
				authData, err := authJSON.Marshal(meta.Internal.AuthData)
				if err != nil {
					return errs.B().Cause(err).Msg("failed to marshal auth data").Err()
				}
				req.SetMeta("AuthData", string(authData))
			}
		}

		// If we're making an internal call, sign the request
		targetAuth := server.internalAuth[targetService.ServiceAuth.Method]
		if targetAuth == nil {
			return errs.B().Msg("no internal auth method configured to talk with target service").Err()
		}
		if err := svcauth.Sign(targetAuth, req); err != nil {
			return errs.B().Cause(err).Msg("failed to sign internal call").Err()
		}
	}

	return nil
}

// MetaFromRequest reads the metadata from the given request and returns it
func (s *Server) MetaFromRequest(req transport.Transport) (meta CallMeta, err error) {
	// Read the meta version if set and check it's only version 1
	// as that's the only version we support
	if metaVersion, found := req.ReadMeta("Version"); found && metaVersion != "1" {
		return CallMeta{}, errors.New("unknown encore meta version")
	}

	// If it was an internal call, read the internal metadata
	if sendingService, found := req.ReadMeta("Internal-Call"); found {
		isInternalCall, err := svcauth.Verify(req, s.internalAuth)
		if err != nil {
			return CallMeta{}, fmt.Errorf("failed to verify internal call: %w", err)
		}
		if !isInternalCall {
			return CallMeta{}, errors.New("no internal call auth found")
		}

		meta.Internal = &InternalCallMeta{
			SendingService: sendingService,
		}

		// Pull the auth data out of the request
		if uid, found := req.ReadMeta("UserID"); found && uid != "" {
			meta.Internal.AuthUID = uid

			if data, found := req.ReadMeta("AuthData"); found && data != "" {
				meta.Internal.AuthData = newAuthDataObj()

				if err := authJSON.Unmarshal([]byte(data), meta.Internal.AuthData); err != nil {
					return CallMeta{}, errs.B().Cause(err).Msg("failed to unmarshal auth data").Err()
				}
			}
		}
	}

	// If we where tracing read the trace ID, span ID
	if traceParent, found := req.ReadMeta(transport.TraceParentKey); found &&
		// For now we only read the traceparent for interanl-to-internal calls, this is because CloudRun
		// is adding a traceparent header to all requests, which is causing our trace system to get confused
		// and think that the initial request is a child of another already traced request
		//
		// In the future we should be able to remove this check and read the traceparent header for all requests
		// to interopt with other tracing systems.
		meta.Internal != nil {
		meta.TraceID, meta.ParentSpanID, _ = parseTraceParent(traceParent)

		if traceState, found := req.ReadMetaValues(transport.TraceStateKey); found {
			meta.ParentEventID, _ = parseTraceState(traceState)
		}
	}

	if correlationID, found := req.ReadMeta(transport.CorrelationIDKey); found {
		// Don't allow arbitrary correlation IDs to be passed through
		if len(meta.CorrelationID) > 64 {
			meta.CorrelationID = correlationID[:64]
		} else {
			meta.CorrelationID = correlationID
		}
	}

	return meta, nil
}

// parseTraceParent parses the trace and span ids from s, which is assumed
// to be in the format of the traceparent header (see https://www.w3.org/TR/trace-context/).
// If it's not a valid traceparent header it returns zero ids and ok == false.
func parseTraceParent(s string) (traceID model.TraceID, spanID model.SpanID, ok bool) {
	const (
		version       = "00"
		traceIDLen    = 32
		spanIDLen     = 16
		traceFlagsLen = 2

		verStart     = 0
		verEnd       = verStart + len(version)
		verSep       = verEnd
		traceIDStart = verSep + 1
		traceIDEnd   = traceIDStart + traceIDLen
		traceIDSep   = traceIDEnd
		spanIDStart  = traceIDSep + 1
		spanIDEnd    = spanIDStart + spanIDLen
		spanIDSep    = spanIDEnd
		flagsStart   = spanIDSep + 1
		flagsEnd     = flagsStart + traceFlagsLen
		totalLen     = flagsEnd
	)

	if len(s) != totalLen || s[verStart:verEnd] != version || s[verSep] != '-' || s[traceIDSep] != '-' || s[spanIDSep] != '-' {
		return model.TraceID{}, model.SpanID{}, false
	}

	_, err := hex.Decode(traceID[:], []byte(s[traceIDStart:traceIDEnd]))
	if err != nil {
		return model.TraceID{}, model.SpanID{}, false
	}

	_, err = hex.Decode(spanID[:], []byte(s[spanIDStart:spanIDEnd]))
	if err != nil {
		return model.TraceID{}, model.SpanID{}, false
	}

	return traceID, spanID, true
}

// parseTraceState parses the trace event id from the tracestate header (see https://www.w3.org/TR/trace-context/).
// If no valid Encore event ID can be parsed it returns zero and ok == false.
//
// Note the spec allows for multiple `tracestate` headers to be sent, so we need to check all of them.
func parseTraceState(headerValues []string) (eventID model.TraceEventID, ok bool) {
	for _, thisHeader := range headerValues {
		theseFields := strings.Split(thisHeader, ",")
		for _, field := range theseFields {
			parts := strings.Split(field, "=")
			if len(parts) != 2 {
				continue
			}

			if parts[0] == eventTraceStateKey {
				eventID, err := strconv.ParseUint(parts[1], 36, 64)
				if err != nil {
					return 0, false
				}

				return model.TraceEventID(eventID), true
			}
		}
	}

	return 0, false
}
