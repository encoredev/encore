package runtime

import (
	"context"
	"sync"
	"time"

	"encore.dev/internal/ctx"
	"encore.dev/runtime/config"
)

var shutdown = struct {
	initiated chan struct{} // closed when graceful shutdown is initiated
	completed chan struct{} // closed when graceful shutdown is completed
	once      sync.Once     // to trigger shutdown logic only once

	mu       sync.Mutex
	handlers []func(force context.Context)
}{
	initiated: make(chan struct{}),
	completed: make(chan struct{}),
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
func RegisterShutdown(fn func(force context.Context)) {
	shutdown.mu.Lock()
	shutdown.handlers = append(shutdown.handlers, fn)
	shutdown.mu.Unlock()
}

func init() {
	go func() {
		<-ctx.App.Done()
		doShutdown()
	}()
}

// doShutdown triggers the shutdown logic.
// If it has already been triggered, it does nothing and returns immediately.
func doShutdown() {
	shutdown.once.Do(func() {
		close(shutdown.initiated)
		if !devMode {
			defaultServer.logger.Info().Msg("got shutdown signal, initiating graceful shutdown")
		}

		var maxWait time.Duration
		if t := config.Cfg.Runtime.ShutdownTimeout; t > 0 {
			maxWait = t
		}
		// we use a new background context here, as we want the shutdown to continue even when ctx.App has been cancelled.
		force, cancel := context.WithTimeout(context.Background(), maxWait)
		defer cancel()

		shutdown.mu.Lock()
		handlers := shutdown.handlers
		shutdown.mu.Unlock()

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
			defaultServer.logger.Info().Msg("shutdown completed")
		}
		close(shutdown.completed)
	})
}
