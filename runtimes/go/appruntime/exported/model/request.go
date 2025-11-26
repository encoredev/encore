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

type TraceEventID uint64

type RequestType byte

const (
	RPCCall       RequestType = 0x01
	AuthHandler   RequestType = 0x02
	PubSubMessage RequestType = 0x03
	Test          RequestType = 0x04
)

type RPCDesc struct {
	Service      string
	SvcNum       uint16
	Endpoint     string
	AuthHandler  bool // true if this is an auth handler
	Raw          bool
	RequestType  reflect.Type // nil if no payload
	ResponseType reflect.Type // nil if no payload
	Tags         []string

	Exposed      bool // True if the endpoint is exposed (access level "public" or "auth")
	AuthRequired bool // True if the endpoint requires authentication ("auth")
}

type PathParams []PathParam

// PathParam represents a parsed path parameter.
type PathParam struct {
	Name  string // the name of the path parameter, without leading ':' or '*'.
	Value string // the parsed path parameter value.
}

type Request struct {
	Type             RequestType
	TraceID          TraceID
	SpanID           SpanID
	ParentSpanID     SpanID
	ParentTraceID    TraceID
	CallerEventID    TraceEventID // the event that triggered this request
	ExtCorrelationID string       // The externally-provided correlation ID, if any.

	Start  time.Time
	Logger *zerolog.Logger
	Traced bool
	DefLoc uint32

	// SvcNum is the 1-based index of the service into the service list.
	// It's here instead of within RPCData/MsgData/Test for performance.
	SvcNum uint16

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

	// ServiceToServiceCall is true if the request was a service-to-service call.
	// otherwise it is false if the request originates from outside the Encore application.
	ServiceToServiceCall bool

	// Mocked is true if the request was handled by a mock.
	Mocked bool
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
	Config  *TestConfig        // The test config (should always be set) and managed by the testsupport Manager

	TestFile string // The file the test is in
	TestLine uint32 // The line the test is on

	// UserID and AuthData are the test-level auth information,
	// if overridden.
	UserID   UID
	AuthData any

	ServiceInstancesMu sync.Mutex
	ServiceInstances   map[string]any // The service instances isolated to this test

	Wait sync.WaitGroup // If we're spun up async go routines, this wait allows to the test to wait for them to end
}

// TestConfig contains configuration for testing,
//
// It can either be the global test config, or a per-test config.
type TestConfig struct {
	// The parent test config, if any.
	//
	// If this is not set, then this test config exists at the global level.
	Parent *TestConfig

	// Lock for the below fields
	Mu sync.RWMutex

	ServiceMocks     map[string]ServiceMock
	APIMocks         map[string]map[string]ApiMock
	IsolatedServices *bool                // Whether to isolate services for this test
	EndCallbacks     []func(t *testing.T) // Callbacks to run when the test ends
}

type ServiceMock struct {
	Service       any
	RunMiddleware bool
}

type ApiMock struct {
	ID            uint64
	Function      any
	RunMiddleware bool
}

type Response struct {
	// HTTPStatus is the HTTP status to respond with.
	HTTPStatus int

	// Duration is how long the request took.
	Duration time.Duration

	// Err is the error returned from the handler or middleware, or nil.
	Err error

	// Typed response payload, for non-raw requests.
	TypedPayload any

	// Payload is the response payload, if any, or nil.
	// It is used for non-raw endpoints as well as auth handlers.
	Payload []byte

	// AuthUID is the resolved user id if this is an auth handler.
	AuthUID UID

	// Headers are HTTP headers to add to the response, set by middleware.
	// For non-raw endpoints, these will be merged with the standard headers.
	Headers http.Header

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
	SpanID SpanID // deprecated: this is not used
	DefLoc uint32

	// Service/endpoint being called
	TargetServiceName  string
	TargetEndpointName string

	// Auth info for the target endpoint
	UserID   UID
	AuthData any

	StartEventID TraceEventID
}

type AuthCall struct {
	ID     uint64 // call id
	SpanID SpanID
	DefLoc uint32
}

type UID string

type AuthInfo struct {
	UID      UID
	UserData any
}

type LogLevel byte

const (
	LevelTrace LogLevel = 0 // unused; reserve for future use
	LevelDebug LogLevel = 1
	LevelInfo  LogLevel = 2
	LevelWarn  LogLevel = 3
	LevelError LogLevel = 4
)

type LogFieldType byte

const (
	ErrField      LogFieldType = 1
	StringField   LogFieldType = 2
	BoolField     LogFieldType = 3
	TimeField     LogFieldType = 4
	DurationField LogFieldType = 5
	UUIDField     LogFieldType = 6
	JSONField     LogFieldType = 7
	IntField      LogFieldType = 8
	UintField     LogFieldType = 9
	Float32Field  LogFieldType = 10
	Float64Field  LogFieldType = 11
)
