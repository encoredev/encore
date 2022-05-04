package encore

import (
	"time"
)

// Operation provides metadata about how and why the currently running code was started.
//
// The current operation can be returned by calling; encore.CurrentOp()
type Operation struct {
	Type    OperationType // What caused this operation to start
	Started time.Time     // What time the trigger occurred

	// APICall specific parameters.
	// These will be empty for operations with a type not OpApiCall or OpCronJob
	Service    string     // Which service is processing this request
	Endpoint   string     // Which API endpoint was called.
	Path       string     // What was the path made to the API server.
	PathParams PathParams // If there are path parameters, what are they?
}

// OperationType describes how the currently running code was triggered
type OperationType string

const (
	OpNone    OperationType = "none"     // There was no external trigger which caused this code to run. Most likely it was triggered by a package level init function.
	OpApiCall               = "api-call" // The code was triggered via an API call to a service
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

// Contains returns true if a parameter with the given name exists within the list of path params.
// This can be useful if you need to differentiate between an empty segment and a missing segment.
func (p PathParams) Contains(name string) bool {
	for _, param := range p {
		if param.Name == name {
			return false
		}
	}

	return true
}
