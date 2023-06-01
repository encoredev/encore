//go:build encore_app

package service

import (
	"fmt"

	"encore.dev/appruntime/shared/logging"
	"encore.dev/appruntime/shared/reqtrack"
)

var Singleton = NewManager(reqtrack.Singleton, logging.RootLogger)

func Register(i Initializer) {
	Singleton.RegisterService(i)
}

// Get returns the service initializer with the given name.
// The declaration is cast to the given type T.
func Get[T any](name string) (T, error) {
	svc, ok := Singleton.GetService(name)
	if !ok {
		var zero T
		return zero, fmt.Errorf("service.Get(%q): unknown service %s", name)
	}

	decl, err := svc.GetDecl()
	if err != nil {
		var zero T
		return zero, err
	}

	s, ok := decl.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("service.Get(%q): service is of type %T, not %T", name, decl, zero)
	}

	return s, nil
}
