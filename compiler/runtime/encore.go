package encore

import (
	"time"

	"encore.dev/beta/auth"
	"encore.dev/runtime"
	"encore.dev/runtime/config"
)

// CurrentRequest reports information about the current request being handled
// by the calling goroutine. If no request is being handled it reports nil.
//
// The only time CurrentRequest reports nil is when calling it from an init function
// or a goroutine spawned from an init function.
// Inside an Encore API endpoint or in code called from that (including in goroutines
// spawned from that) it is safe to assume it reports a non-nil *Request.
//
// It is safe to call concurrently from multiple goroutines and correctly handles
// the same server processing multiple concurrent requests.
func CurrentRequest() *Request {
	req, _, ok := runtime.CurrentRequest()
	if !ok {
		return nil
	}
	return &Request{
		Service:    req.Service,
		Endpoint:   req.Endpoint,
		Start:      req.Start,
		Path:       "",  // TODO
		PathParams: nil, // TODO
		Payload:    nil, // TODO
		UserID:     req.UID,
		UserData:   req.AuthData,
	}
}

// Request represents an incoming HTTP request or API call being handled by Encore.
type Request struct {
	// Service is the name of the service being called.
	Service string
	// Endpoint is the name of the endpoint being called.
	Endpoint string
	// Start is the time when the API endpoint was called to handle the request.
	Start time.Time
	// Path is the HTTP request path for the request.
	Path string
	// PathParams are the path parameters parsed from the request path.
	PathParams PathParams
	// Payload is the request payload parsed from the request body.
	// It is of the same type as the API endpoint's payload parameter.
	Payload any
	// UserID is the authenticated user id for the request, if any.
	// If there is no authenticated user it is "".
	UserID auth.UID
	// UserData reports the auth data returned from the auth handler, if any.
	// It reports nil if there is no authenticated user or if the auth handler does not support auth data.
	UserData any
}

// PathParams contains the path parameters parsed from the request path.
type PathParams []PathParam

// PathParams represents a parsed path parameter.
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

// AppMeta reports metadata about the running Encore application.
// It never returns nil.
func AppMeta() *AppMetadata {
	cfg := config.Cfg
	return &AppMetadata{
		AppID:      cfg.Runtime.AppSlug,
		APIBaseURL: cfg.Runtime.APIBaseURL,
		EnvName:    cfg.Runtime.EnvName,
		EnvType:    EnvType(cfg.Runtime.EnvType),
	}
}

// AppMetadata contains metadata about the running Encore application.
type AppMetadata struct {
	// AppID is the ID of the Encore application.
	// If the application is not linked to the Encore Platform it is "".
	AppID string

	// APIBaseURL is the base URL for calling this API.
	//
	// For local development it is "http://localhost:<port>", typically "http://localhost:4000".
	//
	// If a custom domain is used for this environment it is returned here, but note that
	// changes only take effect at the time of deployment while custom domains can be updated at any time.
	APIBaseURL string

	// EnvName is the name of the environment.
	// For local development it is "local".
	EnvName string

	// EnvType is the type of environment.
	EnvType EnvType
}

// EnvType represents the type of environment.
// Additional environment types may be added in the future.
type EnvType string

const (
	// Production represents a production environment.
	Production EnvType = "production"
	// Development represents a long-lived cloud-hosted, non-production environment,
	// such as test environments.
	Development EnvType = "development"
	// Ephemeral represents short-lived cloud-hosted, non-production environments,
	// such as preview environments that only exist while a particular pull request is open.
	Ephemeral EnvType = "ephemeral"
	// Local represents the local development environment when using 'encore run'.
	Local EnvType = "local"
)
