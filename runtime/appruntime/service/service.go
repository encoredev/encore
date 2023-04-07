package service

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/reqtrack"
	"encore.dev/appruntime/runtimeutil/syncutil"
	"encore.dev/appruntime/trace"
	"encore.dev/beta/errs"
)

// Initializer is a service initializer.
type Initializer interface {
	InitService() error
}

type Decl[T any] struct {
	Service string
	Name    string

	// Setup sets up the service instance.
	// If nil, the service is initialized with new(T).
	Setup func() (*T, error)

	// SetupDefLoc is the location of the Setup function.
	// It is 0 if Setup is nil.
	SetupDefLoc int32

	setupOnce syncutil.Once
	instance  *T // initialized instance, or nil
}

// Get returns the API Decl, initializing it if necessary.
func (g *Decl[T]) Get() (*T, error) {
	err := g.InitService()
	return g.instance, err
}

func (g *Decl[T]) InitService() error {
	return g.setupOnce.Do(func() error { return doSetupService(Singleton, g) })
}

func doSetupService[T any](mgr *Manager, decl *Decl[T]) (err error) {
	curr := mgr.rt.Current()
	if curr.Trace != nil && curr.Req != nil && decl.SetupDefLoc != 0 {
		initCtr := atomic.AddUint64(&mgr.initCounter, 1)
		curr.Trace.ServiceInitStart(trace.ServiceInitStartParams{
			InitCtr: initCtr,
			SpanID:  curr.Req.SpanID,
			Goctr:   curr.Goctr,
			DefLoc:  decl.SetupDefLoc,
			Service: decl.Service,
		})
		defer curr.Trace.ServiceInitEnd(initCtr, err)
	}

	setupFn := decl.Setup
	if setupFn == nil {
		setupFn = func() (*T, error) { return new(T), nil }
	}

	i, err := setupFn()
	if err != nil {
		mgr.rt.Logger().Error().Err(err).Str("service", decl.Service).Msg("service initialization failed")
		return errs.B().Code(errs.Internal).Msg("service initialization failed").Err()
	}
	decl.instance = i

	// If the API Decl supports graceful shutdown, register that with the server.
	if gs, ok := any(i).(shutdowner); ok {
		mgr.registerShutdownHandler(gs)
	}
	return nil
}

// shutdowner is the interface for service structs that
// support graceful shutdown.
type shutdowner interface {
	Shutdown(force context.Context)
}

func NewManager(rt *reqtrack.RequestTracker, rootLogger zerolog.Logger, svcInit []Initializer) *Manager {
	return &Manager{rt: rt, rootLogger: rootLogger, svcInit: svcInit}
}

type Manager struct {
	rt         *reqtrack.RequestTracker
	rootLogger zerolog.Logger
	svcInit    []Initializer

	shutdownMu       sync.Mutex
	shutdownHandlers []shutdowner

	initCounter uint64
}

func (mgr *Manager) InitializeServices() error {
	num := len(mgr.svcInit)
	results := make(chan error, num)

	for _, svc := range mgr.svcInit {
		svc := svc
		go func() {
			err := svc.InitService()
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

func (mgr *Manager) Shutdown(force context.Context) {
	mgr.shutdownMu.Lock()
	handlers := mgr.shutdownHandlers
	mgr.shutdownMu.Unlock()

	var wg sync.WaitGroup
	wg.Add(len(handlers))
	for _, h := range handlers {
		h := h
		go func() {
			defer wg.Done()
			h.Shutdown(force)
		}()
	}

	wg.Wait()
}

func (mgr *Manager) registerShutdownHandler(h shutdowner) {
	mgr.shutdownMu.Lock()
	defer mgr.shutdownMu.Unlock()
	mgr.shutdownHandlers = append(mgr.shutdownHandlers, h)
}

var Singleton *Manager
