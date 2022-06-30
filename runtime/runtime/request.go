package runtime

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"

	"encore.dev/beta/errs"
	"encore.dev/internal/metrics"
	"encore.dev/internal/stack"
	"encore.dev/runtime/config"
	"encore.dev/runtime/trace"

	// These imports are used only by the generated wrappers in the compiler,
	// but add them here so the 'go' command doesn't remove them from go.mod.
	_ "github.com/felixge/httpsnoop"
)

var (
	reqIDCtr  uint32
	callIDCtr uint64
)

var json = jsoniter.Config{
	IndentionStep:          config.JsonIndentStepForResponses(),
	EscapeHTML:             false,
	SortMapKeys:            true,
	ValidateJsonRawMessage: true,
}.Froze()

type UID string

func BeginOperation() {
	encoreBeginOp(true /* always trace */)
}

func FinishOperation() {
	encoreFinishOp()
}

type Type byte

const (
	RPCCall       Type = 0x01
	AuthHandler   Type = 0x02
	PubSubMessage Type = 0x03
	Test          Type = 0x04
)

type Request struct {
	Type     Type
	SpanID   trace.SpanID
	ParentID trace.SpanID
	UID      UID
	AuthData any

	Service      string
	Endpoint     string
	Path         string
	PathSegments httprouter.Params
	MsgData      PubSubMsgData
	Start        time.Time
	Logger       *zerolog.Logger
	Traced       bool

	Test *TestData // If we're running a test, this data represents the test information
}

type RequestData struct {
	Type            Type
	Service         string
	Endpoint        string
	MsgData         PubSubMsgData
	CallExprIdx     int32
	EndpointExprIdx int32
	Inputs          [][]byte
	Path            string
	PathSegments    httprouter.Params
	UID             UID
	AuthData        any
	RequireAuth     bool
}

type PubSubMsgData struct {
	Topic        string
	Subscription string
	MessageID    string
	Published    time.Time
	Attempt      int
}

type TestData struct {
	Ctx     context.Context    // The context we're running for this test
	Cancel  context.CancelFunc // The function to cancel this tests context
	Current *testing.T         // The current test running
	Parent  *Request           // The parent request (if we're looking at sub-tests)

	Wait sync.WaitGroup // If we're spun up async go routines, this wait allows to the test to wait for them to end
}

func BeginRequest(ctx context.Context, data RequestData) error {
	spanID, err := trace.GenSpanID()
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
	SpanID trace.SpanID
}

type CallParams struct {
	Service         string
	Endpoint        string
	CallExprIdx     int32
	EndpointExprIdx int32
}

func BeginCall(params CallParams) (*Call, error) {
	spanID, err := trace.GenSpanID()
	if err != nil {
		return nil, err
	}

	callID := atomic.AddUint64(&callIDCtr, 1)

	if g := encoreGetG(); g != nil && g.req != nil && g.req.data.Traced {
		tb := trace.NewTraceBuf(8 + 4 + 4 + 4)
		tb.UVarint(callID)
		tb.Bytes(g.req.data.SpanID[:])
		tb.Bytes(spanID[:])
		tb.UVarint(uint64(g.goid))
		tb.UVarint(uint64(params.CallExprIdx))
		tb.UVarint(uint64(params.EndpointExprIdx))
		tb.Stack(stack.Build(3))
		encoreTraceEvent(trace.CallStart, tb.Buf())
	}

	return &Call{
		CallID: callID,
		SpanID: spanID,
	}, nil
}

func (c *Call) Finish(err error) {
	if g := encoreGetG(); g != nil && g.req != nil && g.req.data.Traced {
		tb := trace.NewTraceBuf(8 + 4 + 4 + 4)
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
		encoreTraceEvent(trace.CallEnd, tb.Buf())
	}
}

func (c *Call) BeginReq(ctx context.Context, data RequestData) error {
	return beginReq(ctx, c.SpanID, data)
}

func (c *Call) FinishReq(outputs [][]byte, err error) {
	finishReq(outputs, err, 0)
}

type AuthCall struct {
	SpanID trace.SpanID
	CallID uint64
}

func BeginAuth(authHandlerExprIdx int32, token string) (*AuthCall, error) {
	spanID, err := trace.GenSpanID()
	if err != nil {
		return nil, fmt.Errorf("could not generate request id: %v", err)
	}
	callID := atomic.AddUint64(&callIDCtr, 1)

	if g := encoreGetG(); g != nil && g.op.trace != nil {
		tb := trace.NewTraceBuf(8 + 4 + 4 + 4)
		tb.UVarint(callID)
		tb.Bytes(spanID[:])
		tb.UVarint(uint64(g.goid))
		tb.UVarint(uint64(authHandlerExprIdx))
		encoreTraceEvent(trace.AuthStart, tb.Buf())
	}

	return &AuthCall{
		SpanID: spanID,
		CallID: callID,
	}, nil
}

func (ac *AuthCall) Finish(uid UID, err error) {
	if g := encoreGetG(); g != nil && g.op.trace != nil {
		tb := trace.NewTraceBuf(64)
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
		encoreTraceEvent(trace.AuthEnd, tb.Buf())
	}
}

func (ac *AuthCall) BeginReq(ctx context.Context, data RequestData) error {
	return beginReq(ctx, ac.SpanID, data)
}

func (ac *AuthCall) FinishReq(outputs [][]byte, err error) {
	finishReq(outputs, err, 0)
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

func TraceLog(event trace.TraceEvent, data []byte) {
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

func beginReq(ctx context.Context, spanID trace.SpanID, data RequestData) error {
	req := &Request{
		Type:         data.Type,
		SpanID:       spanID,
		Service:      data.Service,
		Endpoint:     data.Endpoint,
		Path:         data.Path,
		PathSegments: data.PathSegments,
		MsgData:      data.MsgData,
		Start:        time.Now(),
		UID:          data.UID,
		AuthData:     data.AuthData,
	}

	if prev, _, ok := currentReq(); ok {
		req.UID = prev.UID
		req.AuthData = prev.AuthData
		req.ParentID = prev.SpanID
		req.Test = prev.Test
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

	logCtx := Logger().With().
		Str("service", req.Service)

	if req.Test != nil {
		logCtx = logCtx.Str("test", req.Test.Current.Name())
	}

	switch req.Type {
	case PubSubMessage:
		logCtx = logCtx.Str("subscription", req.MsgData.Subscription).Str("topic", req.MsgData.Topic).Str("msg-id", req.MsgData.MessageID)
		if req.MsgData.Attempt > 1 {
			logCtx = logCtx.Int("retry", req.MsgData.Attempt-1)
		}
	default:
		logCtx = logCtx.Str("endpoint", req.Endpoint)
	}

	if req.UID != "" {
		logCtx = logCtx.Str("uid", string(req.UID))
	}
	logger := logCtx.Logger()
	req.Logger = &logger

	g := encoreGetG()
	req.Traced = g.op.trace != nil
	if req.Traced {
		tb := trace.NewTraceBuf(1 + 8 + 8 + 8 + 8 + 8 + 8 + 64)
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

		if req.Type == PubSubMessage {
			tb.String(req.MsgData.MessageID)
			tb.Uint32(uint32(req.MsgData.Attempt))
			tb.Time(req.MsgData.Published)
		}

		encoreTraceEvent(trace.RequestStart, tb.Buf())
	}

	switch data.Type {
	case AuthHandler:
		req.Logger.Info().Msg("running auth handler")
	case PubSubMessage:
		req.Logger.Info().Msg("received pubsub message, starting processing")
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

	if req.Traced {
		tb := trace.NewTraceBuf(64)
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
		encoreTraceEvent(trace.RequestEnd, tb.Buf())
	}

	errCode := errs.Code(err).String()
	if httpStatus != 0 {
		errCode = errs.HTTPStatusToCode(httpStatus).String()
	}

	dur := time.Since(req.Start)
	if err != nil {
		e := errs.Convert(err).(*errs.Error)
		ev := req.Logger.Error()
		for k, v := range e.Meta {
			ev = ev.Interface(k, v)
		}
		ev.Str("error", e.ErrorMessage()).Str("code", e.Code.String())

		switch req.Type {
		case AuthHandler:
			ev.Msg("auth handler failed")
		case PubSubMessage:
			ev.Msg("pubsub message processing failed")
		default:
			ev.Msg("request failed")
			metrics.ReqEnd(req.Service, req.Endpoint, dur.Seconds(), e.Code.String())
		}
	} else {
		switch req.Type {
		case AuthHandler:
			req.Logger.Info().Dur("duration", dur).Msg("auth handler completed")
		case PubSubMessage:
			req.Logger.Info().Dur("duration", dur).Msg("pubsub message processed")
		default:
			req.Logger.Info().Dur("duration", dur).Str("code", errCode).Int("http_code", httpStatus).Msg("request completed")
			metrics.ReqEnd(req.Service, req.Endpoint, dur.Seconds(), errCode)
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
