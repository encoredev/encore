package model

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/serde"
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
	UID      UID
	AuthData any

	Service      string
	Endpoint     string
	Path         string
	PathSegments PathParams
	Payload      any
	Inputs       [][]byte
	Start        time.Time
	Logger       *zerolog.Logger
	Traced       bool
	DefLoc       int32

	// Set if Type == RPCCall
	RPCDesc *RPCDesc

	// Set if Type == PubSubMessage
	MsgData *PubSubMsgData

	// If we're running a test, this contains the test information.
	Test *TestData
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

func (i *AuthInfo) Serialize(json jsoniter.API) ([][]byte, error) {
	// BUG(andre) figure out the best way to only dump info.UID
	// if the handler doesn't use the three-result form.
	return serde.SerializeInputs(json, i.UID, i.UserData)
}
