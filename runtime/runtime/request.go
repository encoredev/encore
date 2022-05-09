package runtime

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"

	"encore.dev/beta/errs"
	"encore.dev/internal/metrics"
	"encore.dev/internal/stack"
	"encore.dev/runtime/config"

	// These imports are used only by the generated wrappers in the compiler,
	// but add them here so the 'go' command doesn't remove them from go.mod.
	_ "github.com/felixge/httpsnoop"
)

var (
	reqIDCtr  uint32
	callIDCtr uint64
)

var (
	RootLogger *zerolog.Logger
)

var json = jsoniter.Config{
	IndentionStep:          2,
	EscapeHTML:             false,
	SortMapKeys:            false,
	ValidateJsonRawMessage: true,
}.Froze()

type UID string

func BeginOperation() {
	encoreBeginOp(true /* always trace */)
}

func FinishOperation() {
	encoreFinishOp()
}

type SpanID [8]byte

type Type byte

const (
	RPCCall     Type = 0x01
	AuthHandler Type = 0x02
)

type Request struct {
	Type     Type
	SpanID   SpanID
	ParentID SpanID
	UID      UID
	AuthData any

	Service      string
	Endpoint     string
	Path         string
	PathSegments httprouter.Params
	Start        time.Time
	Logger       zerolog.Logger
	Traced       bool
}

type RequestData struct {
	Type            Type
	Service         string
	Endpoint        string
	CallExprIdx     int32
	EndpointExprIdx int32
	Inputs          [][]byte
	Path            string
	PathSegments    httprouter.Params
	UID             UID
	AuthData        any
	RequireAuth     bool
}

func BeginRequest(ctx context.Context, data RequestData) error {
	spanID, err := genSpanID()
	if err != nil {
		return err
	}
	return beginReq(ctx, spanID, data)
}

func FinishRequest(outputs [][]byte, err error) {
	finishReq(outputs, err, 0)
}

func FinishHTTPRequest(outputs [][]byte, err error, httpStatus int) {
	finishReq(outputs, err, httpStatus)
}

type Call struct {
	CallID uint64
	SpanID SpanID
}

type CallParams struct {
	Service         string
	Endpoint        string
	CallExprIdx     int32
	EndpointExprIdx int32
}

func BeginCall(params CallParams) (*Call, error) {
	spanID, err := genSpanID()
	if err != nil {
		return nil, err
	}

	callID := atomic.AddUint64(&callIDCtr, 1)

	if g := encoreGetG(); g != nil && g.req != nil && g.req.data.Traced {
		tb := NewTraceBuf(8 + 4 + 4 + 4)
		tb.UVarint(callID)
		tb.Bytes(g.req.data.SpanID[:])
		tb.Bytes(spanID[:])
		tb.UVarint(uint64(g.goid))
		tb.UVarint(uint64(params.CallExprIdx))
		tb.UVarint(uint64(params.EndpointExprIdx))
		tb.Stack(stack.Build(3))
		encoreTraceEvent(CallStart, tb.Buf())
	}

	return &Call{
		CallID: callID,
		SpanID: spanID,
	}, nil
}

func (c *Call) Finish(err error) {
	if g := encoreGetG(); g != nil && g.req != nil && g.req.data.Traced {
		tb := NewTraceBuf(8 + 4 + 4 + 4)
		tb.UVarint(c.CallID)
		if err != nil {
			msg := err.Error()
			if msg == "" {
				msg = "unknown error"
			}
			tb.String(msg)
		} else {
			tb.String("")
		}
		encoreTraceEvent(CallEnd, tb.Buf())
	}
}

func (c *Call) BeginReq(ctx context.Context, data RequestData) error {
	return beginReq(ctx, c.SpanID, data)
}

func (c *Call) FinishReq(outputs [][]byte, err error) {
	finishReq(outputs, err, 0)
}

type AuthCall struct {
	SpanID SpanID
	CallID uint64
}

func BeginAuth(authHandlerExprIdx int32, token string) (*AuthCall, error) {
	spanID, err := genSpanID()
	if err != nil {
		return nil, fmt.Errorf("could not generate request id: %v", err)
	}
	callID := atomic.AddUint64(&callIDCtr, 1)

	if g := encoreGetG(); g != nil && g.op.trace != nil {
		tb := NewTraceBuf(8 + 4 + 4 + 4)
		tb.UVarint(callID)
		tb.Bytes(spanID[:])
		tb.UVarint(uint64(g.goid))
		tb.UVarint(uint64(authHandlerExprIdx))
		encoreTraceEvent(AuthStart, tb.Buf())
	}

	return &AuthCall{
		SpanID: spanID,
		CallID: callID,
	}, nil
}

func (ac *AuthCall) Finish(uid UID, err error) {
	if g := encoreGetG(); g != nil && g.op.trace != nil {
		tb := NewTraceBuf(64)
		tb.UVarint(ac.CallID)
		tb.String(string(uid))
		if err != nil {
			msg := err.Error()
			if msg == "" {
				msg = "unknown error"
			}
			tb.String(msg)
			tb.Stack(errs.Stack(err))
		} else {
			tb.String("")
			tb.Stack(stack.Stack{}) // no stack
		}
		encoreTraceEvent(AuthEnd, tb.Buf())
	}
}

func (ac *AuthCall) BeginReq(ctx context.Context, data RequestData) error {
	return beginReq(ctx, ac.SpanID, data)
}

func (ac *AuthCall) FinishReq(outputs [][]byte, err error) {
	finishReq(outputs, err, 0)
}

func Logger() *zerolog.Logger {
	if req, _, ok := CurrentRequest(); ok {
		return &req.Logger
	}
	return RootLogger
}

func CurrentRequest() (*Request, uint32, bool) {
	return currentReq()
}

func currentReq() (*Request, uint32, bool) {
	if g := encoreGetG(); g != nil && g.req != nil {
		return g.req.data, g.goid, true
	}
	return nil, 0, false
}

func TraceLog(event TraceEvent, data []byte) {
	encoreTraceEvent(event, data)
}

func SerializeInputs(inputs ...interface{}) ([][]byte, error) {
	var res [][]byte
	for _, input := range inputs {
		data, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("could not serialize input %v: %v", input, err)
		}
		res = append(res, data)
	}
	return res, nil
}

func CopyInputs(inputs [][]byte, outputs []interface{}) error {
	if len(inputs) != len(outputs) {
		panic(fmt.Sprintf("encore.dev/runtime.CopyInputs: len(inputs) != len(outputs): %v != %v",
			len(inputs), len(outputs)))
	}
	for i, data := range inputs {
		if err := json.Unmarshal(data, outputs[i]); err != nil {
			return fmt.Errorf("could not serialize input #%d: %v", i, err)
		}
	}
	return nil
}

func beginReq(ctx context.Context, spanID SpanID, data RequestData) error {
	req := &Request{
		Type:         data.Type,
		SpanID:       spanID,
		Service:      data.Service,
		Endpoint:     data.Endpoint,
		Path:         data.Path,
		PathSegments: data.PathSegments,
		Start:        time.Now(),
		UID:          data.UID,
		AuthData:     data.AuthData,
	}

	if prev, _, ok := currentReq(); ok {
		req.UID = prev.UID
		req.AuthData = prev.AuthData
		req.ParentID = prev.SpanID
		encoreClearReq()
	}

	// Update request data based on call options, if any
	if opts, _ := ctx.Value(callOptionsKey).(*CallOptions); opts != nil {
		if a := opts.Auth; a != nil {
			if err := checkAuthData(a.UID, a.UserData); err != nil {
				return err
			}
			req.UID = a.UID
			req.AuthData = a.UserData
		}
	}

	if data.RequireAuth && req.UID == "" {
		return &errs.Error{
			Code:    errs.Unauthenticated,
			Message: "endpoint requires auth but none provided",
			Meta: errs.Metadata{
				"service":  req.Service,
				"endpoint": req.Endpoint,
			},
		}
	}

	encoreBeginReq(spanID, req, true /* always trace */)

	logCtx := RootLogger.With().
		Str("service", req.Service).
		Str("endpoint", req.Endpoint)
	if req.UID != "" {
		logCtx = logCtx.Str("uid", string(req.UID))
	}
	req.Logger = logCtx.Logger()

	g := encoreGetG()
	req.Traced = g.op.trace != nil
	if req.Traced {
		tb := NewTraceBuf(1 + 8 + 8 + 8 + 8 + 8 + 8 + 64)
		tb.Bytes([]byte{byte(req.Type)})
		tb.Now()
		tb.Bytes(req.SpanID[:])
		tb.Bytes(req.ParentID[:])
		tb.String(req.Service)
		tb.String(req.Endpoint)
		tb.UVarint(uint64(g.goid))
		tb.UVarint(uint64(data.CallExprIdx))
		tb.UVarint(uint64(data.EndpointExprIdx))
		tb.String(string(req.UID))
		tb.UVarint(uint64(len(data.Inputs)))
		for _, input := range data.Inputs {
			tb.UVarint(uint64(len(input)))
			tb.Bytes(input)
		}
		encoreTraceEvent(RequestStart, tb.Buf())
	}

	switch data.Type {
	case AuthHandler:
		req.Logger.Info().Msg("running auth handler")
	default:
		req.Logger.Info().Msg("starting request")
	}
	return nil
}

func finishReq(outputs [][]byte, err error, httpStatus int) {
	g := encoreGetG()
	if g == nil || g.req == nil {
		panic("encore: no current request running")
	}

	req := g.req.data
	if err != nil {
		switch req.Type {
		case AuthHandler:
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

	if req.Traced {
		tb := NewTraceBuf(64)
		tb.Bytes(req.SpanID[:])
		if err == nil {
			tb.Byte(0) // no error
			tb.UVarint(uint64(len(outputs)))
			for _, output := range outputs {
				tb.UVarint(uint64(len(output)))
				tb.Bytes(output)
			}
		} else {
			tb.Bytes([]byte{1})
			tb.String(err.Error())
			tb.Stack(errs.Stack(err))
		}
		encoreTraceEvent(RequestEnd, tb.Buf())
	}

	dur := time.Since(req.Start)
	switch req.Type {
	case AuthHandler:
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
	encoreCompleteReq()
}

type AuthInfo struct {
	UID      UID
	UserData interface{}
}

type CallOptions struct {
	Auth *AuthInfo
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

func checkAuthData(uid UID, userData interface{}) error {
	if uid == "" && userData != nil {
		return fmt.Errorf("invalid API call options: empty uid and non-empty auth data")
	}

	if authData := config.Cfg.Static.AuthData; authData != nil {
		if uid != "" && userData == nil {
			return fmt.Errorf("invalid API call options: missing auth data")
		} else if userData != nil {
			tt := reflect.TypeOf(userData)
			if tt != authData {
				return fmt.Errorf("invalid API call options: wrong type for auth data (got %s, expected %s)", tt, authData)
			}
		}
	} else {
		if userData != nil {
			return fmt.Errorf("invalid API call options: non-nil auth data (auth handler specifies no auth data)")
		}
	}

	return nil
}
