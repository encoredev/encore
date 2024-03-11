//go:build encore_app

package service

import (
	"fmt"

	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/health"
	"encore.dev/appruntime/shared/logging"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/shutdown"
	"encore.dev/appruntime/shared/testsupport"
)

var Singleton *Manager

func init() {
	Singleton = NewManager(appconf.Static, appconf.Runtime, reqtrack.Singleton, health.Singleton, logging.RootLogger, testsupport.Singleton)
	shutdown.Singleton.RegisterShutdownHandler(Singleton.Shutdown)
}

func Register(i Initializer) {
	Singleton.RegisterService(i)
}

// Get returns the API Decl, initializing it if necessary.
func (g *Decl[T]) Get() (*T, error) {
	if Singleton.static.Testing && Singleton.testMgr.GetIsolatedServices() {
		testData := Singleton.rt.Current().Req.Test

		// Get the instance holder while under lock, but then
		// unlock the mutex before calling doSetupService - as if the service
		// init calls another service which we need to initialize, we would
		// otherwise deadlock.
		testData.ServiceInstancesMu.Lock()
		holderAny, ok := testData.ServiceInstances[g.Service]
		if !ok {
			holderAny = &InstanceHolder[T]{}
			testData.ServiceInstances[g.Service] = holderAny
		}
		testData.ServiceInstancesMu.Unlock()

		// This is a bit of a hack, but we need to cast the holder to the
		// correct type as the TestData struct is in our model package, which
		// we don't want to introduce complex types to
		holder, ok := holderAny.(*InstanceHolder[T])
		if !ok {
			var zero *InstanceHolder[T]
			return nil, fmt.Errorf("failed to cast service instance holder to correct type for service %s. Found %T expected %T", g.Name, holderAny, zero)
		}
		err := holder.setupOnce.Do(func() error {
			return doSetupService(Singleton, g, holder)
		})
		if err != nil {
			return nil, err
		}

		return holder.instance, nil
	}

	err := g.InitService()
	return g.holder.instance, err
}

// GetDecl returns the API Decl, initializing it if necessary.
func (g *Decl[T]) GetDecl() (any, error) {
	return g.Get()
}

func (g *Decl[T]) InitService() error {
	return g.holder.setupOnce.Do(func() error {
		return doSetupService(Singleton, g, &g.holder)
	})
}

// Get returns the service initializer with the given name.
// The declaration is cast to the given type T.
func Get[T any](name string) (T, error) {
	svc, ok := Singleton.GetService(name)
	if !ok {
		var zero T
		return zero, fmt.Errorf("service.Get(%q): unknown service", name)
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
