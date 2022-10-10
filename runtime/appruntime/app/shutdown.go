package app

import (
	"context"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type shutdownTracker struct {
	initiated chan struct{} // closed when graceful shutdown is initiated
	completed chan struct{} // closed when graceful shutdown is completed
	once      sync.Once     // to trigger shutdown logic only once

	mu       sync.Mutex
	handlers []func(force context.Context)
}

func newShutdownTracker() *shutdownTracker {
	return &shutdownTracker{
		initiated: make(chan struct{}),
		completed: make(chan struct{}),
	}
}

// WatchForShutdownSignals watches for shutdown signals (SIGTERM, SIGINT)
// and triggers the graceful shutdown when such a signal is received.
func (app *App) WatchForShutdownSignals() {
	graceful, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-graceful.Done()
		cancel()
		app.Shutdown()
	}()
}

// RegisterShutdown registers a shutdown handler that will be called when the server
// is gracefully shutting down.
//
// The given context is closed when the graceful shutdown window is closed and it's
// time to forcefully shut down. force.Deadline() can be inspected to learn when this
// will happen in advance.
//
// The shutdown is cooperative: the process will not exit until all shutdown handlers
// have returned, unless the process is forcefully killed by a signal (which may happen
// in certain cloud environments if the graceful shutdown takes longer than its timeout).
func (app *App) RegisterShutdown(fn func(force context.Context)) {
	app.shutdown.mu.Lock()
	app.shutdown.handlers = append(app.shutdown.handlers, fn)
	app.shutdown.mu.Unlock()
}

// Shutdown triggers the shutdown logic.
// If it has already been triggered, it does nothing and returns immediately.
func (app *App) Shutdown() {
	app.shutdown.once.Do(func() {
		close(app.shutdown.initiated)
		if !devMode {
			app.rootLogger.Info().Msg("got shutdown signal, initiating graceful shutdown")
		}

		var maxWait time.Duration
		if t := app.cfg.Runtime.ShutdownTimeout; t > 0 {
			maxWait = t
		}
		force, cancel := context.WithTimeout(context.Background(), maxWait)
		defer cancel()

		app.shutdown.mu.Lock()
		handlers := app.shutdown.handlers
		app.shutdown.mu.Unlock()

		// Run our handlers concurrently and wait for them to complete.
		var wg sync.WaitGroup
		wg.Add(len(handlers))
		for _, fn := range handlers {
			fn := fn
			go func() {
				defer wg.Done()
				fn(force)
			}()
		}
		wg.Wait()

		if !devMode {
			app.rootLogger.Info().Msg("shutdown completed")
		}
		close(app.shutdown.completed)
	})
}

// ShutdownInitiated reports whether graceful shutdown has been initiated.
func (app *App) ShutdownInitiated() bool {
	select {
	case <-app.shutdown.initiated:
		return true
	default:
		return false
	}
}
