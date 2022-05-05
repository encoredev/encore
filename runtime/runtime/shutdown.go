package runtime

import (
	"context"
	"os/signal"
	"sync"
	"syscall"
	"time"

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

func RegisterShutdown(fn func(force context.Context)) {
	shutdown.mu.Lock()
	shutdown.handlers = append(shutdown.handlers, fn)
	shutdown.mu.Unlock()
}

func init() {
	graceful, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-graceful.Done()
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
