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

// Get returns the API Decl, initializing it if necessary.
func (g *Decl[T]) Get() (*T, error) {
	err := g.InitService()
	return g.instance, err
}

// GetDecl returns the API Decl, initializing it if necessary.
func (g *Decl[T]) GetDecl() (any, error) {
	if err := g.InitService(); err != nil {
		return nil, err
	}
	return g.instance, nil
}

func (g *Decl[T]) InitService() error {
	return g.setupOnce.Do(func() error { return doSetupService(Singleton, g) })
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
