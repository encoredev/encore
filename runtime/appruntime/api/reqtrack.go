package api

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/julienschmidt/httprouter"

	"encore.dev/appruntime/model"
	"encore.dev/beta/errs"
	"encore.dev/internal/metrics"
)

func (s *Server) beginOperation() {
	s.rt.BeginOperation()
}

func (s *Server) finishOperation() {
	s.rt.FinishOperation()
}

type beginRequestParams struct {
	Type         model.RequestType
	Service      string
	Endpoint     string
	Inputs       [][]byte
	Path         string
	PathSegments httprouter.Params
	UID          model.UID
	AuthData     any
	DefLoc       int32

	// SpanID is the span ID to use.
	// If it is the zero value a new span id is generated.
	SpanID model.SpanID
}

func (s *Server) beginRequest(ctx context.Context, p *beginRequestParams) error {
	spanID := p.SpanID
	if spanID == (model.SpanID{}) {
		id, err := model.GenSpanID()
		if err != nil {
			return err
		}
		spanID = id
	}

	req := &model.Request{
		Type:         p.Type,
		SpanID:       spanID,
		Service:      p.Service,
		Endpoint:     p.Endpoint,
		Path:         p.Path,
		PathSegments: p.PathSegments,
		DefLoc:       p.DefLoc,
		Start:        time.Now(),
		UID:          p.UID,
		AuthData:     p.AuthData,
	}

	logCtx := s.rootLogger.With().Str("service", req.Service).Str("endpoint", req.Endpoint)
	if req.UID != "" {
		logCtx = logCtx.Str("uid", string(req.UID))
	}
	reqLogger := logCtx.Logger()
	req.Logger = &reqLogger

	if prev := s.rt.Current().Req; prev != nil {
		req.UID = prev.UID
		req.AuthData = prev.AuthData
		req.ParentID = prev.SpanID
	}

	// Update request data based on call options, if any
	if opts, _ := ctx.Value(callOptionsKey).(*CallOptions); opts != nil {
		if a := opts.Auth; a != nil {
			authDataType := s.cfg.Static.AuthData
			if err := checkAuthData(authDataType, a.UID, a.UserData); err != nil {
				return err
			}
			req.UID = a.UID
			req.AuthData = a.UserData
		}
	}

	s.rt.BeginRequest(req)
	if curr := s.rt.Current(); curr.Trace != nil {
		curr.Trace.BeginRequest(req, curr.Goctr)
	}

	switch req.Type {
	case model.AuthHandler:
		req.Logger.Info().Msg("running auth handler")
	default:
		req.Logger.Info().Msg("starting request")
	}

	return nil
}

func (s *Server) finishRequest(output [][]byte, err error, httpStatus int) {
	curr := s.rt.Current()
	req := curr.Req
	if req == nil {
		panic("encore: no current request running")
	}

	if err != nil {
		switch req.Type {
		case model.AuthHandler:
			req.Logger.Error().Err(err).Msg("auth handler failed")
		default:
			e := errs.Convert(err).(*errs.Error)
			ev := req.Logger.Error()
			for k, v := range e.Meta {
				ev = ev.Interface(k, v)
			}
			ev.Str("error", e.ErrorMessage()).Str("code", e.Code.String()).Msg("request failed")
		}
	}

	dur := time.Since(req.Start)
	switch req.Type {
	case model.AuthHandler:
		req.Logger.Info().Dur("duration", dur).Msg("auth handler completed")
	default:
		if httpStatus != 0 {
			code := errs.HTTPStatusToCode(httpStatus).String()
			req.Logger.Info().Dur("duration", dur).Str("code", code).Int("http_code", httpStatus).Msg("request completed")
			metrics.ReqEnd(req.Service, req.Endpoint, dur.Seconds(), code)
		} else {
			code := errs.Code(err).String()
			req.Logger.Info().Dur("duration", dur).Str("code", code).Msg("request completed")
			metrics.ReqEnd(req.Service, req.Endpoint, dur.Seconds(), code)
		}
	}

	if curr.Trace != nil {
		curr.Trace.FinishRequest(req, output, err)
	}

	s.rt.FinishRequest()
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

func checkAuthData(authDataType reflect.Type, uid model.UID, userData interface{}) error {
	if uid == "" && userData != nil {
		return fmt.Errorf("invalid API call options: empty uid and non-empty auth data")
	}

	if authDataType != nil {
		if uid != "" && userData == nil {
			return fmt.Errorf("invalid API call options: missing auth data")
		} else if userData != nil {
			tt := reflect.TypeOf(userData)
			if tt != authDataType {
				return fmt.Errorf("invalid API call options: wrong type for auth data (got %s, expected %s)",
					tt, authDataType)
			}
		}
	} else {
		if userData != nil {
			return fmt.Errorf("invalid API call options: non-nil auth data (auth handler specifies no auth data)")
		}
	}

	return nil
}
func (s *Server) beginCall() (*model.APICall, error) {
	spanID, err := model.GenSpanID()
	if err != nil {
		return nil, err
	}

	callID := atomic.AddUint64(&s.callCtr, 1)
	call := &model.APICall{
		ID:     callID,
		SpanID: spanID,
	}

	if curr := s.rt.Current(); curr.Trace != nil {
		curr.Trace.BeginCall(call, curr.Goctr)
	}

	return call, nil
}

func (s *Server) finishCall(call *model.APICall, err error) {
	if curr := s.rt.Current(); curr.Trace != nil {
		curr.Trace.FinishCall(call, err)
	}
}

func (s *Server) beginAuth() (*model.AuthCall, error) {
	spanID, err := model.GenSpanID()
	if err != nil {
		return nil, fmt.Errorf("could not generate request id: %v", err)
	}
	callID := atomic.AddUint64(&s.callCtr, 1)

	call := &model.AuthCall{
		ID:     callID,
		SpanID: spanID,
	}

	if curr := s.rt.Current(); curr.Trace != nil {
		curr.Trace.BeginAuth(call, curr.Goctr)
	}

	return call, nil
}

func (s *Server) finishAuth(call *model.AuthCall, uid model.UID, err error) {
	if curr := s.rt.Current(); curr.Trace != nil {
		curr.Trace.FinishAuth(call, uid, err)
	}
}
