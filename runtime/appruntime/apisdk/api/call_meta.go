package api

import (
	"context"
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
	"encore.dev/appruntime/shared/cloud"
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

const (
	callerMetaName = "Caller"
	calleeMetaName = "Callee"
)

type metaContextKey string

var (
	metaContextKeyAuthInfo metaContextKey = "requestMeta"
)

// CallMeta is metadata for an RPC call
type CallMeta struct {
	// Untrusted fields
	// (i.e. we allow these fields to be passed in from anywhere, including external requests)
	TraceID       model.TraceID      // The trace ID of the calling request (zero if not tracing)
	SpanID        model.SpanID       // The span ID of _this_ request if predefined by the caller (zero in most cases)
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
	Caller   Caller // The name of the service which is making the call
	AuthUID  string // The UID of the authenticated user
	AuthData any    // The data of the authenticated user
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

	// Default the caller to the current app deployment
	var caller Caller
	caller = &AppCaller{s.runtime.DeployID}

	// Trace the source data if we're tracing
	if call != nil && call.Source != nil {
		meta.TraceID = call.Source.TraceID
		meta.ParentSpanID = call.Source.SpanID
		meta.ParentEventID = call.StartEventID
		meta.CorrelationID = call.Source.ExtCorrelationID

		if call.Source.RPCData != nil && call.Source.RPCData.Desc != nil {
			// If we're processing an API call, let's update the caller
			caller = &ApiCaller{
				ServiceName: call.Source.RPCData.Desc.Service,
				Endpoint:    call.Source.RPCData.Desc.Endpoint,
			}
		} else if call.Source.MsgData != nil && call.Source.MsgData.MessageID != "" {
			// If we're processing a PubSub message, let's update the caller
			caller = &PubSubCaller{
				Topic:        call.Source.MsgData.Topic,
				Subscription: call.Source.MsgData.Subscription,
				MessageID:    call.Source.MsgData.MessageID,
			}
		}
	}

	meta.Internal = &InternalCallMeta{
		Caller:   caller,
		AuthUID:  string(call.UserID),
		AuthData: call.AuthData,
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

		if !meta.ParentSpanID.IsZero() {
			// Because Encore does not count an RPC call as a span, but rather a set of events within a span
			// we also need to pass the event ID which started the RPC call in the tracestate header
			eventID := strconv.FormatUint(uint64(meta.ParentEventID), 36)
			if server.runtime.EnvCloud == cloud.GCP || server.runtime.EnvCloud == cloud.Encore {
				// In GCP they add their own span's into the "trace", which breaks our parent span link
				// so we need to add the parent span ID to the tracestate header so we can track our own parent span
				req.SetMeta(transport.TraceStateKey, fmt.Sprintf("%s=%x,%s=%s", eventTraceStateSpanIDKey, meta.ParentSpanID[:], eventTraceStateEventIDKey, eventID))
			} else {
				// Otherwise all we need to know is our event ID
				req.SetMeta(transport.TraceStateKey, fmt.Sprintf("%s=%s", eventTraceStateEventIDKey, eventID))
			}
		}
	}

	// Pass the correlation ID to the downstream service.
	// However, we do _not_ pass the X-Request-ID down, as it is not meant to be propagated through request chains
	if meta.CorrelationID != "" {
		req.SetMeta(transport.CorrelationIDKey, meta.CorrelationID)
	}

	// If we're making an internal call, add the internal metadata to the request
	if meta.Internal != nil {
		// Add a marker to the request to indicate that this is an internal call
		req.SetMeta(callerMetaName, meta.Internal.Caller.CallerString())

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

func (meta CallMeta) IsServiceToService() bool {
	return meta.Internal != nil && meta.Internal.Caller != nil
}

func (meta CallMeta) PrivateAPIAccess() bool {
	return meta.Internal != nil && meta.Internal.Caller != nil && meta.Internal.Caller.PrivateAPIAccess()
}

// MetaFromRequest reads the metadata from the given request and returns it
func (s *Server) MetaFromRequest(req transport.Transport) (meta CallMeta, err error) {
	// Read the meta version if set and check it's only version 1
	// as that's the only version we support
	if metaVersion, found := req.ReadMeta("Version"); found && metaVersion != "1" {
		return CallMeta{}, errors.New("unknown encore meta version")
	}

	// If it was an internal call, read the internal metadata
	if callerStr, found := req.ReadMeta(callerMetaName); found {
		isInternalCall, err := svcauth.Verify(req, s.internalAuth)
		if err != nil {
			return CallMeta{}, fmt.Errorf("failed to verify internal call: %w", err)
		}
		if !isInternalCall {
			return CallMeta{}, errors.New("no internal call auth found")
		}

		caller, err := ParseCallerString(callerStr)
		if err != nil {
			return CallMeta{}, fmt.Errorf("failed to parse caller string: %w", err)
		}

		meta.Internal = &InternalCallMeta{
			Caller: caller,
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
			parentEventID, parentSpanID, ok := parseTraceState(traceState)
			if ok {
				meta.ParentEventID = parentEventID

				// If we where given a parent span ID, use that instead of the one from the traceparent header
				// This is because GCP Cloud Run will add it's own spans in before the application code is run
				// and thus we lose the parent span ID from the traceparent header
				if !parentSpanID.IsZero() {
					meta.ParentSpanID = parentSpanID
				}
			}
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
func parseTraceState(headerValues []string) (eventID model.TraceEventID, parentSpanID model.SpanID, ok bool) {
	for _, thisHeader := range headerValues {
		theseFields := strings.Split(thisHeader, ",")
		for _, field := range theseFields {
			parts := strings.Split(field, "=")
			if len(parts) != 2 {
				continue
			}

			switch parts[0] {
			case eventTraceStateSpanIDKey:
				spanIDBytes, err := hex.DecodeString(parts[1])
				if err != nil || len(spanIDBytes) != 8 {
					return 0, model.SpanID{}, false
				}
				copy(parentSpanID[:], spanIDBytes)

			case eventTraceStateEventIDKey:
				eventIDUint, err := strconv.ParseUint(parts[1], 36, 64)
				if err != nil {
					return 0, model.SpanID{}, false
				}

				eventID = model.TraceEventID(eventIDUint)
			}
		}
	}

	return eventID, parentSpanID, eventID != 0
}

func SetCallMetaInContext(ctx context.Context, meta CallMeta) context.Context {
	return context.WithValue(ctx, metaContextKeyAuthInfo, meta)
}

func CallMetaFromContext(ctx context.Context) CallMeta {
	return ctx.Value(metaContextKeyAuthInfo).(CallMeta)
}
