package svcauth

import (
	"fmt"

	"github.com/benbjohnson/clock"

	"encore.dev/appruntime/apisdk/api/transport"
	"encore.dev/appruntime/exported/config"
)

const (
	AuthMethodMetaKey = "Svc-Auth-Method"
)

// Sign signs the request using the given authentication method.
func Sign(method ServiceAuth, req transport.Transport) error {
	if err := method.sign(req); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}
	req.SetMeta(AuthMethodMetaKey, method.method())

	return nil
}

// Verify verifies the authenticity of the request using the given authentication methods.
func Verify(req transport.Transport, loadedAuthMethods map[string]ServiceAuth) (internalCall bool, err error) {
	method, found := req.ReadMeta(AuthMethodMetaKey)
	if !found {
		// If this is not set, it means that the request is not an internal service to service call.
		return false, nil
	}

	for _, authMethod := range loadedAuthMethods {
		if authMethod.method() == method {
			if err := authMethod.verify(req); err != nil {
				return false, fmt.Errorf("failed to verify request: %w", err)
			}
			return true, nil
		}
	}

	return false, fmt.Errorf("unknown service to service authentication method: %s", method)
}

// LoadMethods loads the service to service authentication methods from the given config.
func LoadMethods(clock clock.Clock, cfg *config.Runtime) (inbound, outbound map[string]ServiceAuth, err error) {
	inbound = make(map[string]ServiceAuth)
	outbound = make(map[string]ServiceAuth)

	load := func(authCfg config.ServiceAuth) (ServiceAuth, error) {
		switch authCfg.Method {
		case "noop":
			return &noop{}, nil
		case "encore-auth":
			return newEncoreAuth(clock, cfg.AppSlug, cfg.EnvName, cfg.AuthKeys), nil
		default:
			return nil, fmt.Errorf("unknown service to service authentication method: %s", authCfg.Method)
		}
	}

	// Load all the inbound auth methods.
	for _, authCfg := range cfg.ServiceAuth {
		inbound[authCfg.Method], err = load(authCfg)
		if err != nil {
			return nil, nil, err
		}
	}

	// Load all the outbound auth methods.
	for _, svc := range cfg.ServiceDiscovery {
		if _, found := outbound[svc.ServiceAuth.Method]; !found {
			outbound[svc.ServiceAuth.Method], err = load(svc.ServiceAuth)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return inbound, outbound, nil
}
