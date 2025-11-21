package test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	encore "encore.dev"
	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/types/option"
	"encore.dev/types/uuid"
)

// Noop allows us to test if a simple HTTP request can be made
//
//encore:api public
func Noop(ctx context.Context) error {
	return nil
}

// NoopWithError allows us to test if the structured errors are returned
//
//encore:api public
func NoopWithError(ctx context.Context) error {
	return &errs.Error{
		Code:    errs.Unimplemented,
		Message: "totally not implemented yet",
	}
}

type BodyEcho struct {
	Message string
}

// SimpleBodyEcho allows us to exercise the body marshalling from JSON
// and being returned purely as a body
//
//encore:api public
func SimpleBodyEcho(ctx context.Context, body *BodyEcho) (*BodyEcho, error) {
	return body, nil
}

var lastMessage = make(map[string]string)

// UpdateMessage allows us to test an API which takes parameters,
// but doesn't return anything
//
//encore:api public method=PUT path=/last_message/:clientID
func UpdateMessage(ctx context.Context, clientID string, message *BodyEcho) error {
	lastMessage[clientID] = message.Message
	return nil
}

// GetMessage allows us to test an API which takes no parameters,
// but returns data. It also tests two API's on the same path with different HTTP methods
//
//encore:api public method=GET path=/last_message/:clientID
func GetMessage(ctx context.Context, clientID string) (*BodyEcho, error) {
	return &BodyEcho{
		Message: lastMessage[clientID],
	}, nil
}

type RestParams struct {
	HeaderValue string `header:"Some-Key"`
	QueryValue  string `query:"Some-Key"`
	BodyValue   string `json:"Some-Key"`

	Nested struct {
		Key   string `json:"Alice"`
		Value int    `json:"bOb"`
		Ok    bool   `json:"charile"`
	}
}

// RestStyleAPI tests all the ways we can get data into and out of the application
// using Encore request handlers
//
//encore:api public method=PUT path=/rest/object/:objType/:name
func RestStyleAPI(ctx context.Context, objType int, name string, params *RestParams) (*RestParams, error) {
	return &RestParams{
		HeaderValue: params.HeaderValue,
		QueryValue:  params.QueryValue,
		BodyValue:   params.BodyValue,
		Nested: struct {
			Key   string `json:"Alice"`
			Value int    `json:"bOb"`
			Ok    bool   `json:"charile"`
		}{
			Key:   name + " + " + params.Nested.Key,
			Value: objType + params.Nested.Value,
			Ok:    params.Nested.Ok,
		},
	}, nil
}

type MarshallerTest[A any] struct {
	HeaderBoolean   bool                  `header:"x-boolean"`
	HeaderInt       int                   `header:"x-int"`
	HeaderFloat     float64               `header:"x-float"`
	HeaderString    string                `header:"x-string"`
	HeaderBytes     []byte                `header:"x-bytes"`
	HeaderTime      time.Time             `header:"x-time"`
	HeaderJson      json.RawMessage       `header:"x-json"`
	HeaderUUID      uuid.UUID             `header:"x-uuid"`
	HeaderUserID    auth.UID              `header:"x-user-id"`
	HeaderOption    option.Option[string] `header:"x-option"`
	QueryBoolean    bool                  `qs:"boolean"`
	QueryInt        int                   `qs:"int"`
	QueryFloat      float64               `qs:"float"`
	QueryString     string                `qs:"string"`
	QueryBytes      []byte                `qs:"bytes"`
	QueryTime       time.Time             `qs:"time"`
	QueryJson       json.RawMessage       `qs:"json"`
	QueryUUID       uuid.UUID             `qs:"uuid"`
	QueryUserID     auth.UID              `qs:"user-id"`
	QuerySlice      []A                   `qs:"slice"`
	BodyBoolean     bool                  `json:"boolean"`
	BodyInt         int                   `json:"int"`
	BodyFloat       float64               `json:"float"`
	BodyString      string                `json:"string"`
	BodyBytes       []byte                `json:"bytes"`
	BodyTime        time.Time             `json:"time"`
	BodyJson        json.RawMessage       `json:"json"`
	BodyUUID        uuid.UUID             `json:"uuid"`
	BodyUserID      auth.UID              `json:"user-id"`
	BodySlice       []A                   `json:"slice"`
	BodyOption      option.Option[A]      `json:"option"`
	BodyOptionSlice []option.Option[A]    `json:"option-slice"`
}

// MarshallerTestHandler allows us to test marshalling of all the inbuilt types in all
// the field types. It simply echos all the responses back to the client
//
//encore:api public
func MarshallerTestHandler(ctx context.Context, params *MarshallerTest[int]) (*MarshallerTest[int], error) {
	return params, nil
}

// TestAuthHandler allows us to test the clients ability to add tokens to requests
//
//encore:api auth
func TestAuthHandler(ctx context.Context) (*BodyEcho, error) {
	userID, ok := auth.UserID()

	return &BodyEcho{
		Message: string(userID) + "::" + strconv.FormatBool(ok),
	}, nil
}

type response struct {
	Body        string
	Header      string
	PathParam   string
	QueryString string
}

// RawEndpoint allows us to test the clients' ability to send raw requests
// under auth
//
//encore:api public raw method=PUT,POST,DELETE,GET path=/raw/blah/*id
func RawEndpoint(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusCreated)

	bytes, err := io.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	req.Body.Close()

	b, err := json.Marshal(&response{
		Body:        string(bytes),
		Header:      req.Header.Get("X-Test-Header"),
		PathParam:   encore.CurrentRequest().PathParams.Get("id"),
		QueryString: req.URL.Query().Get("foo"),
	})
	if err != nil {
		panic(err)
	}
	w.Write(b)
}

type MultiPathSegment struct {
	Boolean  bool
	Int      int
	String   string
	UUID     uuid.UUID
	Wildcard string
}

// PathMultiSegments allows us to wildcard segments and segment URI encoding
//
//encore:api public path=/multi/:bool/:int/:string/:uuid/*wildcard
func PathMultiSegments(ctx context.Context, bool bool, int int, string string, uuid uuid.UUID, wildcard string) (*MultiPathSegment, error) {
	return &MultiPathSegment{
		Boolean:  bool,
		Int:      int,
		String:   string,
		UUID:     uuid,
		Wildcard: wildcard,
	}, nil
}
