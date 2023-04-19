package api

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	model2 "encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/exported/trace"
	"encore.dev/beta/errs"
)

func (s *Server) beginOperation() {
	s.rt.BeginOperation()
}

func (s *Server) finishOperation() {
	s.rt.FinishOperation()
}

type beginRequestParams struct {
	Type   model2.RequestType
	DefLoc int32
	Data   *model2.RPCData

	// TraceID is the trace ID to use.
	// If it is the zero value it will be copied from the parent request.
	TraceID model2.TraceID

	// SpanID is the span ID to use.
	// If it is the zero value a new span id is generated.
	SpanID model2.SpanID

	// ParentTraceID is the correlation ID to use.
	// It is copied from the parent request if it is empty.
	ParentTraceID model2.TraceID

	// ExtRequestID specifies the externally-provided request id, if any.
	// If not empty, it will be recorded as part of the "starting request" log message
	// to facilitate request correlation.
	ExtRequestID string

	// ExtCorrelationID is the externally-provided correlation ID, if any.
	// If not empty, it will be recorded on each log message with "correlation_id" key.
	// to facilitate request correlation.
	ExtCorrelationID string
}

func (s *Server) beginRequest(ctx context.Context, p *beginRequestParams) (*model2.Request, error) {
	spanID := p.SpanID
	if spanID == (model2.SpanID{}) {
		id, err := model2.GenSpanID()
		if err != nil {
			return nil, err
		}
		spanID = id
	}

	req := &model2.Request{
		Type:             p.Type,
		TraceID:          p.TraceID,
		SpanID:           spanID,
		ParentTraceID:    p.ParentTraceID,
		ExtCorrelationID: p.ExtCorrelationID,
		DefLoc:           p.DefLoc,
		SvcNum:           p.Data.Desc.SvcNum,
		Start:            s.clock.Now(),
		Traced:           s.tracingEnabled,
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
		curr.Trace.BeginRequest(req, curr.Goctr)
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

	if req.TraceID != (model2.TraceID{}) {
		logCtx = logCtx.Str("trace_id", req.TraceID.String())
	}

	if req.ExtCorrelationID != "" {
		logCtx = logCtx.Str("x_correlation_id", req.ExtCorrelationID)
	} else if req.ParentTraceID != (model2.TraceID{}) {
		logCtx = logCtx.Str("x_correlation_id", req.ParentTraceID.String())
	}

	reqLogger := logCtx.Logger()
	req.Logger = &reqLogger

	switch req.Type {
	case model2.AuthHandler:
		req.Logger.Info().Msg("running auth handler")
	default:
		ev := req.Logger.Info()
		if p.ExtRequestID != "" {
			ev = ev.Str("ext_request_id", p.ExtRequestID)
		}
		ev.Msg("starting request")
	}

	return req, nil
}

func (s *Server) finishRequest(resp *model2.Response) {
	curr := s.rt.Current()
	req := curr.Req
	if req == nil {
		panic("encore: no current request running")
	}

	if resp.Err != nil {
		switch req.Type {
		case model2.AuthHandler:
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

	dur := time.Since(req.Start)
	switch req.Type {
	case model2.AuthHandler:
		req.Logger.Info().Dur("duration", dur).Msg("auth handler completed")
	default:
		if resp.HTTPStatus != errs.HTTPStatus(resp.Err) {
			code := errs.HTTPStatusToCode(resp.HTTPStatus).String()
			req.Logger.Info().Dur("duration", dur).Str("code", code).Int("http_code", resp.HTTPStatus).Msg("request completed")
		} else {
			code := errs.Code(resp.Err).String()
			req.Logger.Info().Dur("duration", dur).Str("code", code).Msg("request completed")
		}
	}

	if curr.Trace != nil {
		// Capture the recorded bytes from the request and response body, if any.
		if len(resp.RawRequestPayload) > 0 {
			curr.Trace.BodyStream(trace.BodyStreamParams{
				SpanID:     req.SpanID,
				IsResponse: false,
				Overflowed: resp.RawRequestPayloadOverflowed,
				Data:       resp.RawRequestPayload,
			})
		}

		if len(resp.RawResponsePayload) > 0 {
			curr.Trace.BodyStream(trace.BodyStreamParams{
				SpanID:     req.SpanID,
				IsResponse: true,
				Overflowed: resp.RawResponsePayloadOverflowed,
				Data:       resp.RawResponsePayload,
			})
		}

		curr.Trace.FinishRequest(req, resp)
	}

	s.requestsTotal.With(requestsTotalLabels{
		endpoint: req.RPCData.Desc.Endpoint,
		code:     code(resp.Err, resp.HTTPStatus),
	}).Increment()
	s.rt.FinishRequest()
}

type CallOptions struct {
	Auth *model2.AuthInfo
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

// CheckAuthData checks whether the given auth information is valid
// based on the configured auth handler's data type.
func CheckAuthData(uid model2.UID, userData any) error {
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

func (s *Server) beginCall(defLoc int32) (*model2.APICall, error) {
	spanID, err := model2.GenSpanID()
	if err != nil {
		return nil, err
	}

	callID := atomic.AddUint64(&s.callCtr, 1)
	call := &model2.APICall{
		ID:     callID,
		SpanID: spanID,
		DefLoc: defLoc,
	}

	curr := s.rt.Current()
	call.Source = curr.Req

	if curr.Trace != nil {
		curr.Trace.BeginCall(call, curr.Goctr)
	}

	return call, nil
}

func (s *Server) finishCall(call *model2.APICall, err error) {
	if curr := s.rt.Current(); curr.Trace != nil {
		curr.Trace.FinishCall(call, err)
	}
}

func (s *Server) beginAuth(defLoc int32) (*model2.AuthCall, error) {
	spanID, err := model2.GenSpanID()
	if err != nil {
		return nil, fmt.Errorf("could not generate request id: %v", err)
	}
	callID := atomic.AddUint64(&s.callCtr, 1)

	call := &model2.AuthCall{
		ID:     callID,
		SpanID: spanID,
		DefLoc: defLoc,
	}

	if curr := s.rt.Current(); curr.Trace != nil {
		curr.Trace.BeginAuth(call, curr.Goctr)
	}

	return call, nil
}

func (s *Server) finishAuth(call *model2.AuthCall, uid model2.UID, err error) {
	if curr := s.rt.Current(); curr.Trace != nil {
		curr.Trace.FinishAuth(call, uid, err)
	}
}
