package api

import (
	"context"

	"encore.dev/appruntime/runtimeutil/syncutil"
	"encore.dev/beta/errs"
)

type Group[T any] struct {
	Service string
	Name    string

	// Setup sets up the API Group.
	// If nil, the service is initialized with new(T).
	Setup func() (*T, error)

	setupOnce syncutil.Once
	instance  *T // initialized instance, or nil
}

// Get returns the API Group, initializing it if necessary.
func (g *Group[T]) Get() (*T, error) {
	err := g.setupOnce.Do(func() error {
		i, err := g.doSetup()
		if err != nil {
			Singleton.rt.Logger().Error().Err(err).Str("apigroup", g.Name).Msg("api group initialization failed")
			return errs.B().Code(errs.Internal).Msg("service initialization failed").Err()
		}
		g.instance = i

		// If the API Group supports graceful shutdown, register that with the server.
		if gs, ok := any(i).(gracefulShutdowner); ok {
			Singleton.registerShutdownHandler(gs)
		}
		return nil
	})
	return g.instance, err
}

func (g *Group[T]) doSetup() (*T, error) {
	if g.Setup == nil {
		return new(T), nil
	}
	return g.Setup()
}

type gracefulShutdowner interface {
	Shutdown(force context.Context)
}
