-- go.mod --
module app

-- encore.app --
{"id": ""}

-- svc/svc.go --
package svc

import (
    "encoding/json"
    "time"

    "encore.dev/beta/auth"
    "encore.dev/types/uuid"
    "encore.dev/types/option"
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

    QueryFoo bool    `query:"foo" encore:"optional"`
    QueryBar string  `query:"bar" encore:"optional"`
    HeaderBaz string `header:"baz" encore:"optional"`
    HeaderInt int    `header:"int" encore:"optional"`

// This is a multiline
    // comment on the raw message!
    Raw json.RawMessage
}

type WrappedRequest = Wrapper[Request]

type GetRequest struct {
    Bar string `qs:"-"`
    Baz int    `qs:"boo"`
}

// Foo represents a documented integer type
type Foo int

// DocumentedUser represents a user in the system with profile information
type DocumentedUser struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

// DocumentedOrder represents a customer order with references
type DocumentedOrder struct {
    // Customer who placed this order (different from shipping recipient)
    Customer DocumentedUser `json:"customer"`
    OrderID  string         `json:"order_id"`

    OptionalRef option.Option[DocumentedUser] `json:"opt_ref"`
    RequiredRef *DocumentedUser `json:"req_ref"`
}

type Nested struct {
    Value string
}

type AllInputTypes[A any] struct {
    A time.Time `header:"X-Alice"`               // Specify this comes from a header field
    B []int     `query:"Bob"`                    // Specify this comes from a query string
    C bool      `json:"Charlies-Bool,omitempty"` // This can come from anywhere, but if it comes from the payload in JSON it must be called Charile
    Dave A                                       // This generic type complicates the whole thing 🙈

    Optional option.Option[A] `json:"optional"` // An optional generic type

    // Tags named "-" are ignored in schemas
    Ignore1 string `header:"-"`
    Ignore2 string `query:"-"`
    Ignore3 string `json:"-"`

    // Unexported tags are ignored
    ignore4 string
}

// HeaderOnlyStruct contains all types we support in headers
type HeaderOnlyStruct struct {
    Boolean bool            `header:"x-boolean"`
    Int     int             `header:"x-int"`
    Float   float64         `header:"x-float"`
    String  string          `header:"x-string"`
    Bytes   []byte          `header:"x-bytes"`
    Time    time.Time       `header:"x-time"`
    Json    json.RawMessage `header:"x-json"`
    UUID    uuid.UUID       `header:"x-uuid"`
    UserID  auth.UID        `header:"x-user-id"`
    Optional option.Option[string] `header:"x-optional"`
}

type Recursive struct {
    Optional *Recursive `encore:"optional"`
    Slice   []Recursive
    SliceOfOptional   []option.Option[*Recursive]
    Map map[string]Recursive
    MapOfOptional map[string]option.Option[*Recursive]
}

-- svc/api.go --
package svc

import (
    "context"
    "net/http"
    "app/svc/nested"
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


//encore:api public path=/webhook2/:a/*b
func Webhook2(ctx context.Context, a string, b string) error { return nil }

//encore:api public path=/fallbackPath/:a/!b
func FallbackPath(ctx context.Context, a string, b string) error { return nil }

//encore:api public method=POST
func RequestWithAllInputTypes(ctx context.Context, req *AllInputTypes[string]) (*AllInputTypes[float64], error) {
    return nil
}

//encore:api public method=GET
func GetRequestWithAllInputTypes(ctx context.Context, req *AllInputTypes[int]) (HeaderOnlyStruct, error) {
    return nil
}

//encore:api public method=GET
func HeaderOnlyRequest(ctx context.Context, req *HeaderOnlyStruct) error {
    return nil
}

type WithNested struct {
    Nested *nested.Type
}

//encore:api public method=POST
func Nested(ctx context.Context, req *WithNested) (*WithNested, error) {
    return nil, nil
}

//encore:api public method=POST
func Rec(ctx context.Context, req *Recursive) (*Recursive, error) {
    return nil, nil
}

//encore:api public method=POST
func CreateDocumentedOrder(ctx context.Context, req *DocumentedOrder) (*DocumentedOrder, error) {
    return req, nil
}
-- svc/nested/nested.go --
package nested

type Type struct {
    Message string
}

-- products/product.go --
package products

import (
    "context"
    "time"

    "encore.dev/types/uuid"

    "app/authentication"
)

type Product struct {
    ID          uuid.UUID            `json:"id"`
    Name        string               `json:"name"`
    Description string               `json:"description,omitempty"`
    CreatedAt   time.Time            `json:"created_at"`
    CreatedBy   *authentication.User `json:"created_by"`
}

type ProductListing struct {
    Products []*Product `json:"products"`

    PreviousPage struct {
        Cursor string `json:"cursor,omitempty"`
        Exists bool   `json:"exists"`
    } `json:"previous"`

    NextPage struct {
        Cursor string `json:"cursor,omitempty"`
        Exists bool   `json:"exists"`
    } `json:"next"`
}

type CreateProductRequest struct {
    IdempotencyKey string `header:"Idempotency-Key"`
    Name           string `json:"name"`
    Description    string `json:"description,omitempty"`
}

//encore:api public method=GET
func List(ctx context.Context) (*ProductListing, error) {
    return nil, nil
}

//encore:api auth
func Create(ctx context.Context, req *CreateProductRequest) (*Product, error) {
    return nil, nil
}

-- authentication/auth.go --
package authentication

import (
    "context"

    "encore.dev/beta/auth"
)

type AuthData struct {
    APIKey string `header:"X-API-Key"`
}

type User struct {
    ID   int     `json:"id"`
    Name string  `json:"name"`
}

//encore:authhandler
func AuthenticateRequest(ctx context.Context, auth *AuthData) (auth.UID, *User, error) {
    return "", nil, nil
}

// FooType docs
type FooType struct {
    Moo string // Moo docs
    Bar BarType // Bar docs
}

// BarType docs
type BarType struct {
    Baz string // Baz docs
}

//encore:api public
func Docs(ctx context.Context, req *FooType) error {
    return nil
}
