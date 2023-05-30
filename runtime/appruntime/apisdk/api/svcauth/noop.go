package svcauth

import (
	"encore.dev/appruntime/apisdk/api/transport"
)

// noop is a ServiceAuth implementation that does not perform any authentication.
//
// It is intended to be used for local development or where services are running within their own
// private network and there is no threat model resulting in the need to authenticate requests.
type noop struct{}

var Noop noop

var _ ServiceAuth = noop{}

func (n noop) method() string {
	return "noop"
}

func (n noop) verify(transport.Transport) error {
	return nil
}

func (n noop) sign(transport.Transport) error {
	return nil
}
