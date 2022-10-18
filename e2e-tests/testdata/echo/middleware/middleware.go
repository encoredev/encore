package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"encore.dev/beta/errs"
	"encore.dev/middleware"
)

//encore:middleware target=tag:error
func ErroringMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
	return middleware.Response{
		Err: errs.B().Code(errs.Internal).Msg("middleware error").Err(),
	}
}

//encore:middleware target=tag:resp-rewrite
func ResponseRewritingMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
	resp := next(req)
	reqPayload := req.Data().Payload.(*Payload)
	respPayload := resp.Payload.(*Payload)

	respPayload.Msg = fmt.Sprintf("middleware(req=%s, resp=%s)",
		reqPayload.Msg, respPayload.Msg)
	return resp
}

//encore:middleware target=tag:resp-gen
func ResponseGeneratingMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
	responseData := `{"Msg": "middleware generated"}`
	payload := reflect.New(req.Data().API.ResponseType)
	if err := json.Unmarshal([]byte(responseData), payload.Interface()); err != nil {
		return middleware.Response{Err: err}
	}
	return middleware.Response{Payload: payload.Elem().Interface()}
}

type Payload struct {
	Msg string
}

//encore:api public tag:error
func Error(ctx context.Context) error {
	return nil
}

//encore:api public tag:resp-rewrite
func ResponseRewrite(ctx context.Context, req *Payload) (*Payload, error) {
	return &Payload{Msg: fmt.Sprintf("handler(%s)", req.Msg)}, nil
}

//encore:api public tag:resp-gen
func ResponseGen(ctx context.Context, msg *Payload) (*Payload, error) {
	return nil, nil
}
