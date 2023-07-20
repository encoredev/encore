package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/appruntime/shared/cfgutil"
	"encore.dev/appruntime/shared/health"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/syncutil"
	"encore.dev/beta/errs"
)

// Initializer is a service initializer.
type Initializer interface {
	// ServiceName reports the name of the service.
	ServiceName() string

	// InitService initializes the service.
	InitService() error

	// GetDecl returns the service declaration,
	// initializing it if necessary.
	GetDecl() (any, error)
}

type Decl[T any] struct {
	Service string
	Name    string

	// Setup sets up the service instance.
	// If nil, the service is initialized with new(T).
	Setup func() (*T, error)

	// SetupDefLoc is the location of the Setup function.
	// It is 0 if Setup is nil.
	SetupDefLoc uint32

	setupOnce syncutil.Once
	instance  *T // initialized instance, or nil
}

func (g *Decl[T]) ServiceName() string {
	return g.Service
}

func doSetupService[T any](mgr *Manager, decl *Decl[T]) (err error) {
	curr := mgr.rt.Current()
	if curr.Trace != nil && curr.Req != nil && decl.SetupDefLoc != 0 {
		eventParams := trace2.EventParams{
			TraceID: curr.Req.TraceID,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			DefLoc:  decl.SetupDefLoc,
		}
		startID := curr.Trace.ServiceInitStart(trace2.ServiceInitStartParams{
			EventParams: eventParams,
			Service:     decl.Service,
		})
		defer curr.Trace.ServiceInitEnd(eventParams, startID, err)
	}

	setupFn := decl.Setup
	if setupFn == nil {
		setupFn = func() (*T, error) { return new(T), nil }
	}

	decl.instance, err = setupFn()
	if err != nil {
		mgr.rt.Logger().Error().Err(err).Str("service", decl.Service).Msg("service initialization failed")
		return errs.B().Code(errs.Internal).Msgf("service %s: initialization failed", decl.Service).Err()
	}

	// If the API Decl supports graceful shutdown, register that with the server.
	if gs, ok := any(decl.instance).(shutdowner); ok {
		mgr.registerShutdownHandler(serviceShutdown{decl.Service, gs})
	}
	return nil
}

// shutdowner is the interface for service structs that
// support graceful shutdown.
type shutdowner interface {
	Shutdown(force context.Context)
}

type serviceShutdown struct {
	name     string
	instance shutdowner
}

func NewManager(runtime *config.Runtime, rt *reqtrack.RequestTracker, healthChecks *health.CheckRegistry, rootLogger zerolog.Logger) *Manager {
	mgr := &Manager{rt: rt, runtime: runtime, rootLogger: rootLogger, svcMap: make(map[string]Initializer), initialisedServices: make(map[string]struct{})}

	// Register with the health check service.
	healthChecks.Register(mgr)

	return mgr
}

type Manager struct {
	runtime    *config.Runtime
	rt         *reqtrack.RequestTracker
	rootLogger zerolog.Logger
	svcInit    []Initializer
	svcMap     map[string]Initializer

	initialisedMu       sync.RWMutex
	initialisedServices map[string]struct{}

	shutdownMu       sync.Mutex
	shutdownHandlers []serviceShutdown
}

func (mgr *Manager) RegisterService(i Initializer) {
	name := i.ServiceName()
	if !cfgutil.IsHostedService(mgr.runtime, name) {
		return
	}

	if _, ok := mgr.svcMap[name]; ok {
		panic(fmt.Sprintf("service %s: already registered", name))
	}
	mgr.svcMap[name] = i
	mgr.svcInit = append(mgr.svcInit, i)
}

func (mgr *Manager) InitializeServices() error {
	num := len(mgr.svcInit)
	results := make(chan error, num)

	for _, svc := range mgr.svcInit {
		svc := svc
		go func() {
			err := svc.InitService()
			if err == nil {
				mgr.initialisedMu.Lock()
				defer mgr.initialisedMu.Unlock()
				mgr.initialisedServices[svc.ServiceName()] = struct{}{}
			}
			results <- err
		}()
	}

	for i := 0; i < num; i++ {
		if err := <-results; err != nil {
			return err
		}
	}

	return nil
}

// HealthCheck returns a failure if any services have not yet been initialized.
//
// This allows the health check service to report that the server is not yet
// ready to serve requests.
func (mgr *Manager) HealthCheck(ctx context.Context) []health.CheckResult {
	mgr.initialisedMu.RLock()
	defer mgr.initialisedMu.RUnlock()

	// If all services have been initialized, return a single check result.
	if len(mgr.initialisedServices) == len(mgr.svcMap) {
		return []health.CheckResult{{Name: "services.initialized"}}
	}

	// Build a list of services that have not been initialized.
	uninitializedServices := make([]string, 0, len(mgr.svcMap)-len(mgr.initialisedServices))
	for svc := range mgr.svcMap {
		if _, ok := mgr.initialisedServices[svc]; !ok {
			uninitializedServices = append(uninitializedServices, svc)
		}
	}
	sort.Strings(uninitializedServices)

	// Return an error listing the names of each service not yet initialized.
	return []health.CheckResult{{
		Name: "services.initialized",
		Err:  fmt.Errorf("the following services have not returned from their initService functions: %s", strings.Join(uninitializedServices, ", ")),
	}}
}

func (mgr *Manager) GetService(name string) (i Initializer, ok bool) {
	i, ok = mgr.svcMap[name]
	return i, ok
}

func (mgr *Manager) Shutdown(force context.Context) {
	doLog := mgr.runtime.EnvCloud != "local"

	mgr.shutdownMu.Lock()
	handlers := mgr.shutdownHandlers
	mgr.shutdownMu.Unlock()

	var wg sync.WaitGroup
	wg.Add(len(handlers))
	for _, h := range handlers {
		h := h
		go func() {
			defer wg.Done()

			if doLog {
				mgr.rootLogger.Trace().Str("service", h.name).Msg("shutting down service...")
				defer func() {
					if r := recover(); r != nil {
						mgr.rootLogger.Error().Str("service", h.name).Interface("panic", r).Msg("service shutdown panicked")
					} else {
						mgr.rootLogger.Trace().Str("service", h.name).Msg("service shutdown complete")
					}
				}()
			}
			h.instance.Shutdown(force)
		}()
	}

	wg.Wait()
}

func (mgr *Manager) registerShutdownHandler(h serviceShutdown) {
	mgr.shutdownMu.Lock()
	defer mgr.shutdownMu.Unlock()
	mgr.shutdownHandlers = append(mgr.shutdownHandlers, h)
}
