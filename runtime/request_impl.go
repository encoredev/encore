//go:build encore_internal

package encore

import (
	"time"

	"encore.dev/runtime"
)

var applicationStartTime = time.Now()

// CurrentRequest returns the Request that is currently being handled by the calling Go routine
//
// It is safe for concurrent use and will return a new Request on each evocation, so can be mutated by the
// calling code without impacting future calls.
//
// CurrentRequest never returns nil.
func CurrentRequest() *Request {
	req, _, inRequest := runtime.CurrentRequest()
	if !inRequest {
		return &Request{
			Type:    None,
			Started: applicationStartTime,
		}
	}

	opType := None
	switch req.Type {
	case runtime.RPCCall, runtime.AuthHandler:
		opType = APICall
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
