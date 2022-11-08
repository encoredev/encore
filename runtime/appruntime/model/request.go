package model

import (
	"context"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type RequestType byte

const (
	RPCCall       RequestType = 0x01
	AuthHandler   RequestType = 0x02
	PubSubMessage RequestType = 0x03
	Test          RequestType = 0x04
)

type RPCDesc struct {
	Service      string
	Endpoint     string
	AuthHandler  bool // true if this is an auth handler
	Raw          bool
	RequestType  reflect.Type // nil if no payload
	ResponseType reflect.Type // nil if no payload
}

type PathParams []PathParam

// PathParam represents a parsed path parameter.
type PathParam struct {
	Name  string // the name of the path parameter, without leading ':' or '*'.
	Value string // the parsed path parameter value.
}

type Request struct {
	Type     RequestType
	SpanID   SpanID
	ParentID SpanID

	Start  time.Time
	Logger *zerolog.Logger
	Traced bool
	DefLoc int32

	// Set if Type == RPCCall
	RPCData *RPCData

	// Set if Type == PubSubMessage
	MsgData *PubSubMsgData

	// If we're running a test, this contains the test information.
	Test *TestData
}

// Service reports the current service, if any.
// It checks the appropriate sub-structure based on the request type.
func (req *Request) Service() string {
	switch req.Type {
	case RPCCall, AuthHandler:
		return req.RPCData.Desc.Service
	case PubSubMessage:
		return req.MsgData.Service
	default:
		if req.Test != nil {
			return req.Test.Service
		}
		return ""
	}
}

type RPCData struct {
	Desc *RPCDesc

	// HTTPMethod is the HTTP method used to call the endpoint.
	// It is not set for auth handlers.
	HTTPMethod string

	Path       string
	PathParams PathParams

	// UserID and AuthData are the authentication information
	// provided for this endpoint. It is not set for auth handlers
	// as the information is not available yet.
	UserID   UID
	AuthData any

	// Decoded request payload, for non-raw requests
	TypedPayload any

	// NonRawPayload is the JSON-marshalled request payload, if any, or nil.
	// This is never set for raw requests, as the body hasn't been read yet.
	NonRawPayload []byte

	// RequestHeaders contains the HTTP headers from the incoming request.
	RequestHeaders http.Header

	// FromEncorePlatform specifies whether the request was an
	// authenticated request from the Encore Platform.
	FromEncorePlatform bool
}

type PubSubMsgData struct {
	Service        string
	Topic          string
	Subscription   string
	MessageID      string
	Published      time.Time
	Attempt        int
	DecodedPayload any
	// Payload is the JSON-encoded payload.
	Payload []byte
}

type TestData struct {
	Ctx     context.Context    // The context we're running for this test
	Cancel  context.CancelFunc // The function to cancel this tests context
	Current *testing.T         // The current test running
	Parent  *Request           // The parent request (if we're looking at sub-tests)
	Service string             // the service being tested, if any

	// UserID and AuthData are the test-level auth information,
	// if overridden.
	UserID   UID
	AuthData any

	Wait sync.WaitGroup // If we're spun up async go routines, this wait allows to the test to wait for them to end
}

type Response struct {
	// HTTPStatus is the HTTP status to respond with.
	HTTPStatus int

	// Err is the error returned from the handler or middleware, or nil.
	Err error

	// Typed response payload, for non-raw requests.
	TypedPayload any

	// Payload is the response payload, if any, or nil.
	// It is used for non-raw endpoints as well as auth handlers.
	Payload []byte

	// AuthUID is the resolved user id if this is an auth handler.
	AuthUID UID

	// RawRequestPayload contains the captured request payload, for raw requests.
	// It is nil if nothing was captured.
	RawRequestPayload           []byte
	RawRequestPayloadOverflowed bool // whether the payload overflowed

	// RawResponsePayload contains the captured response payload, for raw requests.
	// It is nil if nothing was captured.
	RawResponsePayload           []byte
	RawResponsePayloadOverflowed bool // whether the payload overflowed

	// RawResponseHeaders specifies the HTTP headers for the outgoing response,
	// for raw endpoints only.
	RawResponseHeaders http.Header
}

type APICall struct {
	ID     uint64 // call id
	Source *Request
	SpanID SpanID
	DefLoc int32
}

type AuthCall struct {
	ID     uint64 // call id
	SpanID SpanID
	DefLoc int32
}

type UID string

type AuthInfo struct {
	UID      UID
	UserData any
}
