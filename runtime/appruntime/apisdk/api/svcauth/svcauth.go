package svcauth

import (
	"encore.dev/appruntime/apisdk/api/transport"
)

// ServiceAuth is an interface that provides authentication for internal service to service
// calls within the same Encore application.
type ServiceAuth interface {
	// Method returns the name of the authentication method.
	method() string

	// Verify verifies the authenticity of the request.
	// If the request is not authentic, an error is returned.
	verify(req transport.Transport) error

	// Sign signs the request.
	// If the request cannot be signed, an error is returned.
	sign(req transport.Transport) error
}
