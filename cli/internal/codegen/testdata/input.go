-- go.mod --
module app

-- encore.app --
{"id": ""}

-- svc/svc.go --
package svc

import (
    "encoding/json"
    "time"
)

type UnusedType struct {
Foo Foo
}

type Wrapper[T any] struct { Value T }

// Tuple is a generic type which allows us to
// return two values of two different types
type Tuple[A any, B any] struct {
  A A
  B B
}

type Request struct {
    Foo Foo    `encore:"optional"` // Foo is good
    Bar string `json:"-"`
    Baz string `json:"boo"`        // Baz is better

    // This is a multiline
    // comment on the raw message!
    Raw json.RawMessage
}

type WrappedRequest = Wrapper[Request]

type GetRequest struct {
    Bar string `qs:"-"`
    Baz int    `qs:"boo"`
}

type Foo int

type Nested struct {
    Value string
}

type AllInputTypes[A any] struct {
    A []time.Time         `header:"X-Alice"`       // Specify this comes from a header field
    B int                 `query:"Bob"`              // Specify this comes from a query string
    C bool                `json:"Charile,omitempty"` // This can come from anywhere, but if it comes from the payload in JSON it must be called Charile
    Dave A                                           // This generic type complicates the whole thing 🙈
}

type HeaderOnlyStruct struct {
    Foo []int `header:"X-Foo"`
    Bar bool `header:"X-Bar"`
}

-- svc/api.go --
package svc

import (
    "context"
    "net/http"
)

// DummyAPI is a dummy endpoint.
//encore:api public
func DummyAPI(ctx context.Context, req *Request) error {
    return nil
}

//encore:api public method=GET
func Get(ctx context.Context, req *GetRequest) error {
    return nil
}

// TupleInputOutput tests the usage of generics in the client generator
// and this comment is also multiline, so multiline comments get tested as well.
//encore:api public
func TupleInputOutput(ctx context.Context, req Tuple[string, WrappedRequest]) (Tuple[bool, Foo], error) {
    return nil
}

//encore:api public path=/path/:a/:b
func RESTPath(ctx context.Context, a string, b int) error {
    return nil
}

//encore:api public raw path=/webhook/:a/*b
func Webhook(w http.ResponseWriter, req *http.Request) {}

//encore:api public method=POST
func RequestWithAllInputTypes(ctx context.Context, req *AllInputTypes[string]) (*AllInputTypes[float64], error) {
    return nil
}

//encore:api public method=GET
func GetRequestWithAllInputTypes(ctx context.Context, req *AllInputTypes[int]) (HeaderOnlyStruct, error) {
    return nil
}
