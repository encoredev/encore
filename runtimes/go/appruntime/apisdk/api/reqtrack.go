package api

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/beta/errs"
)

func (s *Server) beginOperation() {
	s.rt.BeginOperation()
}

func (s *Server) finishOperation() {
	s.rt.FinishOperation()
}

type beginRequestParams struct {
	Type   model.RequestType
	DefLoc uint32
	Data   *model.RPCData

	// TraceID is the trace ID to use.
	// If it is the zero value it will be copied from the parent request.
	TraceID model.TraceID

	// SpanID is the span ID to use.
	// If it is the zero value a new span id is generated.
	SpanID model.SpanID

	// ParentTraceID is the parent trace ID to use for correlation.
	// It is copied from the parent request if it is empty.
	ParentTraceID model.TraceID

	// ParentSpanID is the parent's span ID to use for correlation.
	// It is copied from the parent request if it is empty.
	ParentSpanID model.SpanID

	// ParentSampled indicates whether the parent span sampled trace information.
	ParentSampled bool

	// CallerEventID is the event ID in the parent span that triggered this request.
	// It's used to correlate the request with the originating call.
	CallerEventID model.TraceEventID

	// ExtRequestID specifies the externally-provided request id, if any.
	// If not empty, it will be recorded as part of the "starting request" log message
	// to facilitate request correlation.
	ExtRequestID string

	// ExtCorrelationID is the externally-provided correlation ID, if any.
	// If not empty, it will be recorded on each log message with "correlation_id" key.
	// to facilitate request correlation.
	ExtCorrelationID string

	// AdditionalLogFields is a map of additional fields to be added to all the log message.
	// This is mainly used to add the trace identifiers to the log messages
	// so the clouds logging can correlate the logs with the trace.
	AdditionalLogFields map[string]string
}

func (s *Server) beginRequest(ctx context.Context, p *beginRequestParams) (*model.Request, error) {
	traceID := p.TraceID
	if traceID.IsZero() {
		id, err := model.GenTraceID()
		if err != nil {
			return nil, err
		}
		traceID = id
	}

	spanID := p.SpanID
	if spanID.IsZero() {
		id, err := model.GenSpanID()
		if err != nil {
			return nil, err
		}
		spanID = id
	}

	var traced bool
	if p.ParentSpanID.IsZero() {
		traced = s.rt.SampleTrace()
	} else {
		traced = p.ParentSampled
	}

	req := &model.Request{
		Type:             p.Type,
		TraceID:          traceID,
		SpanID:           spanID,
		ParentSpanID:     p.ParentSpanID,
		ParentTraceID:    p.ParentTraceID,
		CallerEventID:    p.CallerEventID,
		ExtCorrelationID: p.ExtCorrelationID,
		DefLoc:           p.DefLoc,
		SvcNum:           p.Data.Desc.SvcNum,
		Start:            s.clock.Now(),
		Traced:           traced,
		RPCData:          p.Data,
	}

	data := req.RPCData

	// Update request data based on call options, if any
	if opts, _ := ctx.Value(callOptionsKey).(*CallOptions); opts != nil {
		if a := opts.Auth; a != nil {
			if err := CheckAuthData(a.UID, a.UserData); err != nil {
				return nil, fmt.Errorf("invalid API call options: %v", err)
			}
			data.UserID = a.UID
			data.AuthData = a.UserData
		}
	}

	// Begin the request, copying data over from the previous request.
	s.rt.BeginRequest(req)
	if curr := s.rt.Current(); curr.Trace != nil {
		switch req.Type {
		case model.RPCCall:
			curr.Trace.RequestSpanStart(req, curr.Goctr)
		case model.AuthHandler:
			curr.Trace.AuthSpanStart(req, curr.Goctr)
		case model.PubSubMessage:
			curr.Trace.PubsubMessageSpanStart(req, curr.Goctr)
		}
	}

	// Now that we have up-to-date information in req (possibly copied from
	// the parent request), construct our logger.
	desc := req.RPCData.Desc
	logCtx := s.rootLogger.With().Str("service", desc.Service).Str("endpoint", desc.Endpoint)
	if data.UserID != "" {
		logCtx = logCtx.Str("uid", string(data.UserID))
	}

	if req.Test != nil {
		logCtx = logCtx.Str("test", req.Test.Current.Name())
	}

	if req.TraceID != (model.TraceID{}) {
		logCtx = logCtx.Str("trace_id", req.TraceID.String())
	}

	if req.ExtCorrelationID != "" {
		logCtx = logCtx.Str("x_correlation_id", req.ExtCorrelationID)
	} else if req.ParentTraceID != (model.TraceID{}) {
		logCtx = logCtx.Str("x_correlation_id", req.ParentTraceID.String())
	}

	// Add additional log fields, if any
	for k, v := range p.AdditionalLogFields {
		logCtx = logCtx.Str(k, v)
	}

	reqLogger := logCtx.Logger()
	req.Logger = &reqLogger

	switch req.Type {
	case model.AuthHandler:
		req.Logger.Trace().Msg("running auth handler")
	default:
		ev := req.Logger.Trace()
		if p.ExtRequestID != "" {
			ev = ev.Str("ext_request_id", p.ExtRequestID)
		}
		ev.Msg("starting request")
	}

	return req, nil
}

func (s *Server) finishRequest(resp *model.Response) {
	curr := s.rt.Current()
	req := curr.Req
	if req == nil {
		panic("encore: no current request running")
	}

	if resp.Err != nil {
		switch req.Type {
		case model.AuthHandler:
			req.Logger.Error().Err(resp.Err).Msg("auth handler failed")
		default:
			e := errs.Convert(resp.Err).(*errs.Error)
			ev := req.Logger.Error()

			var panicStack *stack.Stack
			for k, v := range e.Meta {
				if k == "panic_stack" {
					if st, ok := v.(stack.Stack); ok {
						panicStack = &st
					}
					continue
				}
				ev = ev.Interface(k, v)
			}

			if panicStack != nil {
				ev = ev.Interface("stack", stack.Format(*panicStack))
			}

			ev.Str("error", e.ErrorMessage()).Str("code", e.Code.String()).Msg("request failed")
		}
	}

	resp.Duration = time.Since(req.Start)
	switch req.Type {
	case model.AuthHandler:
		req.Logger.Trace().Dur("duration", resp.Duration).Msg("auth handler completed")
	default:
		if resp.HTTPStatus != errs.HTTPStatus(resp.Err) {
			code := errs.HTTPStatusToCode(resp.HTTPStatus).String()
			req.Logger.Trace().Dur("duration", resp.Duration).Str("code", code).Int("http_code", resp.HTTPStatus).Msg("request completed")
		} else {
			code := errs.Code(resp.Err).String()
			req.Logger.Trace().Dur("duration", resp.Duration).Str("code", code).Msg("request completed")
		}
	}

	if curr.Trace != nil {
		// Capture the recorded bytes from the request and response body, if any.
		if len(resp.RawRequestPayload) > 0 {
			curr.Trace.BodyStream(trace2.BodyStreamParams{
				EventParams: trace2.EventParams{
					TraceID: req.TraceID,
					SpanID:  req.SpanID,
				},
				IsResponse: false,
				Overflowed: resp.RawRequestPayloadOverflowed,
				Data:       resp.RawRequestPayload,
			})
		}

		if len(resp.RawResponsePayload) > 0 {
			curr.Trace.BodyStream(trace2.BodyStreamParams{
				EventParams: trace2.EventParams{
					TraceID: req.TraceID,
					SpanID:  req.SpanID,
				},
				IsResponse: true,
				Overflowed: resp.RawResponsePayloadOverflowed,
				Data:       resp.RawResponsePayload,
			})
		}

		ep := trace2.EventParams{TraceID: req.TraceID, SpanID: req.SpanID}
		switch req.Type {
		case model.RPCCall:
			curr.Trace.RequestSpanEnd(trace2.RequestSpanEndParams{
				EventParams: ep,
				Req:         req,
				Resp:        resp,
			})
		case model.AuthHandler:
			curr.Trace.AuthSpanEnd(trace2.AuthSpanEndParams{
				EventParams: ep,
				Req:         req,
				Resp:        resp,
			})
		case model.PubSubMessage:
			curr.Trace.PubsubMessageSpanEnd(trace2.PubsubMessageSpanEndParams{
				EventParams: ep,
				Req:         req,
				Resp:        resp,
			})
		}
	}

	s.requestsTotal.With(requestsTotalLabels{
		endpoint: req.RPCData.Desc.Endpoint,
		code:     Code(resp.Err, resp.HTTPStatus),
	}).Increment()
	s.rt.FinishRequest(false)
}

type CallOptions struct {
	Auth *model.AuthInfo
}

type ctxKey string

const callOptionsKey ctxKey = "call"

func WithCallOptions(ctx context.Context, opts *CallOptions) context.Context {
	return context.WithValue(ctx, callOptionsKey, opts)
}

func GetCallOptions(ctx context.Context) *CallOptions {
	if opts, _ := ctx.Value(callOptionsKey).(*CallOptions); opts != nil {
		return opts
	}
	return &CallOptions{}
}

// RegisteredAuthDataType is the reflect type of the auth handler's data type.
//
// If no auth handler is configured, this is nil.
// If an auth handler is configured, this is a pointer to the auth handler's data type.
var RegisteredAuthDataType reflect.Type

// newAuthDataObj returns a new instance of the configured auth handler's data type.
// If no auth handler is configured, nil is returned.
func newAuthDataObj() any {
	if RegisteredAuthDataType == nil {
		return nil
	}
	return reflect.New(RegisteredAuthDataType.Elem()).Interface()
}

// CheckAuthData checks whether the given auth information is valid
// based on the configured auth handler's data type.
func CheckAuthData(uid model.UID, userData any) error {
	if uid == "" && userData != nil {
		return fmt.Errorf("empty uid and non-empty auth data")
	}

	if RegisteredAuthDataType != nil {
		tt := reflect.TypeOf(userData)
		if uid != "" && userData == nil {
			return fmt.Errorf("missing auth data (auth handler specifies auth data of type %s)", tt)
		} else if userData != nil {
			if tt != RegisteredAuthDataType {
				return fmt.Errorf("wrong type for auth data (got %s, expected %s)",
					tt, RegisteredAuthDataType)
			}
		}
	} else {
		if userData != nil {
			return fmt.Errorf("unexpected auth data provided (auth handler specifies no auth data)")
		}
	}

	return nil
}

func (s *Server) beginCall(ctx context.Context, serviceName, endpointName string, defLoc uint32) (*model.APICall, CallMeta, error) {
	call := &model.APICall{
		TargetServiceName:  serviceName,
		TargetEndpointName: endpointName,
		DefLoc:             defLoc,
	}

	curr := s.rt.Current()
	call.Source = curr.Req

	// Add  auth data to the call, if any
	if curr.Req != nil && curr.Req.RPCData != nil {
		call.UserID = curr.Req.RPCData.UserID
		call.AuthData = curr.Req.RPCData.AuthData
	}

	// Update request data based on call options, if any
	if opts, _ := ctx.Value(callOptionsKey).(*CallOptions); opts != nil {
		if a := opts.Auth; a != nil {
			call.UserID = a.UID
			call.AuthData = a.UserData
		}
	}

	if curr.Trace != nil {
		call.StartEventID = curr.Trace.RPCCallStart(call, curr.Goctr)
	}

	meta, err := s.metaFromAPICall(call)
	if err != nil {
		return nil, CallMeta{}, err
	}

	return call, meta, nil
}

func (s *Server) finishCall(call *model.APICall, err error) {
	if curr := s.rt.Current(); curr.Trace != nil && call.StartEventID != 0 {
		curr.Trace.RPCCallEnd(call, curr.Goctr, err)
	}
}

func (s *Server) beginAuth(defLoc uint32) (*model.AuthCall, error) {
	spanID, err := model.GenSpanID()
	if err != nil {
		return nil, fmt.Errorf("could not generate request id: %v", err)
	}
	callID := atomic.AddUint64(&s.callCtr, 1)

	call := &model.AuthCall{
		ID:     callID,
		SpanID: spanID,
		DefLoc: defLoc,
	}

	return call, nil
}
