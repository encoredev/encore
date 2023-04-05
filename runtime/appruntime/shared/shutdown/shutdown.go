package shutdown

import (
	"context"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
)

type Tracker struct {
	runtime *config.Runtime
	logger  zerolog.Logger

	initiated chan struct{} // closed when graceful shutdown is initiated
	completed chan struct{} // closed when graceful shutdown is completed
	once      sync.Once     // to trigger shutdown logic only once

	mu       sync.Mutex
	handlers []func(force context.Context)
}

func NewTracker(runtime *config.Runtime, logger zerolog.Logger) *Tracker {
	return &Tracker{
		runtime:   runtime,
		logger:    logger,
		initiated: make(chan struct{}),
		completed: make(chan struct{}),
	}
}

// WatchForShutdownSignals watches for shutdown signals (SIGTERM, SIGINT)
// and triggers the graceful shutdown when such a signal is received.
func (t *Tracker) WatchForShutdownSignals() {
	if t.runtime.EnvType == "test" {
		// Do nothing during tests.
		return
	}

	graceful, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-graceful.Done()
		cancel()
		t.Shutdown()
	}()
}

// OnShutdown registers a shutdown handler that will be called when the app
// is gracefully shutting down.
//
// The given context is closed when the graceful shutdown window is closed and it's
// time to forcefully shut down. force.Deadline() can be inspected to learn when this
// will happen in advance.
//
// The shutdown is cooperative: the process will not exit until all shutdown handlers
// have returned, unless the process is forcefully killed by a signal (which may happen
// in certain cloud environments if the graceful shutdown takes longer than its timeout).
//
// If t is nil this function is a no-op.
func (t *Tracker) OnShutdown(fn func(force context.Context)) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.handlers = append(t.handlers, fn)
}

// Shutdown triggers the shutdown logic.
// If it has already been triggered, it does nothing and returns immediately.
func (t *Tracker) Shutdown() {
	t.once.Do(func() {
		close(t.initiated)

		doLog := t.runtime.EnvCloud != "local"
		if doLog {
			t.logger.Info().Msg("got shutdown signal, initiating graceful shutdown")
		}

		var maxWait time.Duration
		if t := t.runtime.ShutdownTimeout; t > 0 {
			maxWait = t
		}
		force, cancel := context.WithTimeout(context.Background(), maxWait)
		defer cancel()

		t.mu.Lock()
		handlers := t.handlers
		t.mu.Unlock()

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

		if doLog {
			t.logger.Info().Msg("shutdown completed")
		}
		close(t.completed)
	})
}

// ShutdownInitiated reports whether graceful shutdown has been initiated.
func (t *Tracker) ShutdownInitiated() bool {
	select {
	case <-t.initiated:
		return true
	default:
		return false
	}
}
