package model

import (
	"context"
	"sync"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"
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

type Request struct {
	Type     RequestType
	SpanID   SpanID
	ParentID SpanID
	UID      UID
	AuthData any

	Service      string
	Endpoint     string
	Path         string
	PathSegments httprouter.Params
	Inputs       [][]byte // TODO figure out if this makes sense
	Start        time.Time
	Logger       *zerolog.Logger
	Traced       bool
	DefLoc       int32

	// If this is a pubsub message, this contains information about it.
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
}

type AuthCall struct {
	ID     uint64 // call id
	SpanID SpanID
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
