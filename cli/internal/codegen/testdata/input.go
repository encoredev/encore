-- go.mod --
module app

-- encore.app --
{"id": ""}

-- svc/svc.go --
package svc

import "encoding/json"

type UnusedType struct {
Foo Foo
}

type Wrapper[T any] = T

type Tuple[A any, B any] struct {
  A A
  B B
}

type Request struct {
    Foo Foo `encore:"optional"`
    Bar string `json:"-"`
    Baz string `json:"boo"`
    Raw json.RawMessage
}

type WrappedRequest = Wrapper[Request]

type GetRequest struct {
    Bar string `qs:"-"`
    Baz string `qs:"boo"`
}

type Foo int

-- svc/api.go --
package svc

import "context"

//encore:api public
func DummyAPI(ctx context.Context, req *Request) error {
    return nil
}

//encore:api public method=GET
func Get(ctx context.Context, req *GetRequest) error {
    return nil
}

//encore:api public
func TupleInputOutput(ctx context.Context, req Tuple[string, WrappedRequest]) (Tuple[bool, Foo], error) {
    return nil
}

//encore:api public path=/path/:a/:b
func RESTPath(ctx context.Context, a string, b int) error {
    return nil
}
