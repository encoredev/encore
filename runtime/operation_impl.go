//go:build encore_internal

package encore

import (
	"time"

	"encore.dev/runtime"
)

var applicationStartTime = time.Now()

// CurrentOp returns the Operation which was the root cause of why the current code is running.
//
// It is thread safe and will return a new Operation on each evocation, so can be mutated by the
// calling code without impacting future calls.
//
// CurrentOp will never return nil.
func CurrentOp() *Operation {
	req, _, inRequest := runtime.CurrentRequest()
	if !inRequest {
		return &Operation{
			Type:    OpNone,
			Started: applicationStartTime,
		}
	}

	opType := OpNone
	switch req.Type {
	case runtime.RPCCall, runtime.AuthHandler:
		opType = OpApiCall
	}

	pathParams := make(PathParams, len(req.PathSegments))
	for i, param := range req.PathSegments {
		pathParams[i].Name = param.Key
		pathParams[i].Value = param.Value
	}

	return &Operation{
		Type:       opType,
		Service:    req.Service,
		Endpoint:   req.Endpoint,
		Started:    req.Start,
		Path:       req.Path,
		PathParams: pathParams,
	}
}
