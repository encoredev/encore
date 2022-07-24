package encore

import (
	"time"

	"encore.dev/appruntime/model"
)

var applicationStartTime = time.Now()

// Request provides metadata about how and why the currently running code was started.
//
// The current request can be returned by calling CurrentRequest()
type Request struct {
	Type    RequestType // What caused this request to start
	Started time.Time   // What time the trigger occurred

	// APICall specific parameters.
	// These will be empty for operations with a type not APICall
	Service    string     // Which service is processing this request
	Endpoint   string     // Which API endpoint was called.
	Path       string     // What was the path made to the API server.
	PathParams PathParams // If there are path parameters, what are they?
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

	opType := None
	switch req.Type {
	case model.RPCCall, model.AuthHandler:
		opType = APICall
	case model.PubSubMessage:
		opType = PubSubMessage
	}

	pathParams := make(PathParams, len(req.PathSegments))
	for i, param := range req.PathSegments {
		pathParams[i].Name = param.Key
		pathParams[i].Value = param.Value
	}

	return &Request{
		Type:       opType,
		Service:    req.Service,
		Endpoint:   req.Endpoint,
		Started:    req.Start,
		Path:       req.Path,
		PathParams: pathParams,
	}
}
