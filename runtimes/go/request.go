package encore

import (
	"net/http"
	"reflect"
	"slices"
	"time"

	"encore.dev/appruntime/exported/model"
)

var applicationStartTime = time.Now()

// APIDesc describes the API endpoint being called.
type APIDesc struct {
	// RequestType specifies the type of the request payload,
	// or nil if the endpoint has no request payload or is Raw.
	RequestType reflect.Type

	// ResponseType specifies the type of the response payload,
	// or nil if the endpoint has no response payload or is Raw.
	ResponseType reflect.Type

	// Raw specifies whether the endpoint is a Raw endpoint.
	Raw bool

	// Tags describes what tags are attached to the endpoint.
	Tags Tags

	// Exposed is true if the endpoint is exposed to the public internet.
	// This is true for "public" and "auth" endpoints.
	Exposed bool

	// AuthRequired is true if the endpoint requires authentication to be called.
	// This is true for "auth" endpoints.
	AuthRequired bool
}

// Request provides metadata about how and why the currently running code was started.
//
// The current request can be returned by calling CurrentRequest()
type Request struct {
	Type    RequestType // What caused this request to start
	Started time.Time   // What time the trigger occurred

	// Trace contains the trace information for the current request.
	Trace *TraceData

	// APICall specific parameters.
	// These will be empty for operations with a type not APICall
	API        *APIDesc   // Metadata about the API endpoint being called
	Service    string     // Which service is processing this request
	Endpoint   string     // Which API endpoint is being called
	Path       string     // What was the path made to the API server
	PathParams PathParams // If there are path parameters, what are they?
	Method     string     // What HTTP method was used

	// Headers contains the request headers sent with the request, if any.
	//
	// It is currently empty for service-to-service API calls when the caller
	// and callee are both running within the same process.
	// This behavior may change in the future.
	Headers http.Header

	// PubSubMessage specific parameters.
	// Message contains information about the PubSub message,
	Message *MessageData

	// Payload is the decoded request payload or Pub/Sub message payload,
	// or nil if the API endpoint has no request payload or the endpoint is raw.
	Payload any

	// CronIdempotencyKey contains a unique id for a particular cron job execution
	// if this request was triggered by a Cron Job.
	//
	// It can be used to uniquely identify a particular Cron Job execution event,
	// and also serves as a way to distinguish between Cron Job-triggered requests
	// and other requests.
	//
	// If the request was not triggered by a Cron Job the value is the empty string.
	CronIdempotencyKey string
}

// TraceData describes the trace information for a request.
type TraceData struct {
	TraceID          string
	SpanID           string
	ParentTraceID    string // empty if no parent trace
	ParentSpanID     string // empty if no parent span
	ExtCorrelationID string // empty if no correlation id
	Recorded         bool   // true if this trace is being recorded
}

// MessageData describes the request data for a Pub/Sub message.
type MessageData struct {
	// Service is the name of the service with the subscription.
	Service string

	// Topic is the name of the topic the message was published to.
	Topic string

	// Subscription is the name of the subscription the message was received on.
	Subscription string

	// ID is the unique ID of the message assigned by the messaging service.
	// It is the same value returned by topic.Publish() and is the same
	// across delivery attempts.
	ID string

	// Published is the time the message was first published.
	Published time.Time

	// DeliveryAttempt is a counter for how many times the messages
	// has been attempted to be delivered.
	DeliveryAttempt int
}

// RequestType describes how the currently running code was triggered
type RequestType string

const (
	None          RequestType = "none"           // There was no external trigger which caused this code to run. Most likely it was triggered by a package level init function.
	APICall       RequestType = "api-call"       // The code was triggered via an API call to a service
	PubSubMessage RequestType = "pubsub-message" // The code was triggered by a PubSub subscriber
)

// PathParams contains the path parameters parsed from the request path.
// The ordering of the parameters in the path will be maintained from the URL.
type PathParams []PathParam

// PathParam represents a parsed path parameter.
type PathParam struct {
	Name  string // the name of the path parameter, without leading ':' or '*'.
	Value string // the parsed path parameter value.
}

// Get returns the value of the path parameter with the given name.
// If no such parameter exists it reports "".
func (p PathParams) Get(name string) string {
	for _, param := range p {
		if param.Name == name {
			return param.Value
		}
	}

	return ""
}

func (mgr *Manager) CurrentRequest() *Request {
	req := mgr.rt.Current().Req
	if req == nil {
		return &Request{
			Type:    None,
			Started: applicationStartTime,
		}
	}

	result := &Request{
		Started: req.Start,
		Trace: &TraceData{
			TraceID:          req.TraceID.String(),
			SpanID:           req.SpanID.String(),
			ParentTraceID:    req.ParentTraceID.String(),
			ParentSpanID:     req.ParentSpanID.String(),
			ExtCorrelationID: req.ExtCorrelationID,
			Recorded:         req.Traced,
		},
	}

	switch req.Type {
	case model.RPCCall, model.AuthHandler:
		data := req.RPCData
		desc := data.Desc

		result.Type = APICall
		result.Service = desc.Service
		result.Endpoint = desc.Endpoint
		result.Payload = data.TypedPayload

		result.Path = data.Path
		result.PathParams = make(PathParams, len(data.PathParams))
		for i, param := range data.PathParams {
			result.PathParams[i].Name = param.Name
			result.PathParams[i].Value = param.Value
		}
		result.Method = data.HTTPMethod
		result.Headers = data.RequestHeaders

		result.API = &APIDesc{
			RequestType:  desc.RequestType,
			ResponseType: desc.ResponseType,
			Raw:          desc.Raw,
			Tags:         desc.Tags,
			Exposed:      desc.Exposed,
			AuthRequired: desc.AuthRequired,
		}

		if data.FromEncorePlatform {
			result.CronIdempotencyKey = data.RequestHeaders.Get("X-Encore-Cron-Execution")
		}

	case model.PubSubMessage:
		result.Type = PubSubMessage
		result.Service = req.MsgData.Service
		result.Payload = req.MsgData.DecodedPayload
		result.Message = &MessageData{
			Service:         req.MsgData.Service,
			Topic:           req.MsgData.Topic,
			Subscription:    req.MsgData.Subscription,
			ID:              req.MsgData.MessageID,
			Published:       req.MsgData.Published,
			DeliveryAttempt: req.MsgData.Attempt,
		}
	}

	return result
}

// Tags describes a set of tags an endpoint is tagged with,
// without the "tag:" prefix.
//
// The ordering is unspecified.
type Tags []string

// Has reports whether the set contains the given tag.
// The provided value should not contain the "tag:" prefix.
func (tags Tags) Has(tag string) bool {
	return slices.Contains(tags, tag)
}
