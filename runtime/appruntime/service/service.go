package service

import (
	"context"
	"sync"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/runtimeutil/syncutil"
	"encore.dev/beta/errs"
)

type Decl[T any] struct {
	Service string
	Name    string

	// Setup sets up the service instance.
	// If nil, the service is initialized with new(T).
	Setup func() (*T, error)

	setupOnce syncutil.Once
	instance  *T // initialized instance, or nil
}

// Get returns the API Decl, initializing it if necessary.
func (g *Decl[T]) Get() (*T, error) {
	err := g.setupOnce.Do(func() error {
		i, err := g.doSetup()
		if err != nil {
			Singleton.rootLogger.Error().Err(err).Msg("service initialization failed")
			return errs.B().Code(errs.Internal).Msg("service initialization failed").Err()
		}
		g.instance = i

		// If the API Decl supports graceful shutdown, register that with the server.
		if gs, ok := any(i).(shutdowner); ok {
			Singleton.registerShutdownHandler(gs)
		}
		return nil
	})
	return g.instance, err
}

func (g *Decl[T]) doSetup() (*T, error) {
	if g.Setup == nil {
		return new(T), nil
	}
	return g.Setup()
}

// shutdowner is the interface for service structs that
// support graceful shutdown.
type shutdowner interface {
	Shutdown(force context.Context)
}

func NewManager(rootLogger zerolog.Logger) *Manager {
	return &Manager{rootLogger: rootLogger}
}

type Manager struct {
	rootLogger zerolog.Logger

	shutdownMu       sync.Mutex
	shutdownHandlers []shutdowner
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
