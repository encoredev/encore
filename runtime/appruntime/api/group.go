package api

import (
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
func (d *Group[T]) Get() (*T, error) {
	err := d.setupOnce.Do(func() error {
		if d.Setup == nil {
			d.instance = new(T)
			return nil
		}

		var err error
		d.instance, err = d.Setup()
		if err != nil {
			Singleton.rt.Logger().Error().Err(err).Str("apigroup", d.Name).Msg("api group initialization failed")
			return errs.B().Code(errs.Internal).Msg("service initialization failed").Err()
		}
		return err
	})
	return d.instance, err
}
