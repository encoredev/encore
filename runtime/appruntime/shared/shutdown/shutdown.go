package shutdown

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/beta/errs"
)

// Hook are functions which have registered to perform cooperative
// shutdown tasks. They are called concurrently when the server wants
// to perform a graceful shutdown.
//
// The force context will be canceled when the server is going to give
// up on the hook returning and will proceed to forcefully shutdown
// the instance.
//
// Hooks are called instantly when the server is starting a graceful
// shutdown, and are expected to block until they are done.
type Hook func(force context.Context)

// HandlerHook are functions which have registered to be notified
// when contexts passed to currently running handlers should be cancelled.
//
// HandlerHooks are only called after the Handler timeout has occurred.
// These functions are expected to block until all active handlers have
// returned.
type HandlerHook func()

type Tracker struct {
	logger zerolog.Logger

	watchSignals         bool
	logShutdown          bool
	gracefulTimeout      time.Duration
	handlerTimeout       time.Duration
	shutdownHooksTimeout time.Duration

	initiated chan struct{} // closed when graceful shutdown is initiated
	once      sync.Once     // to trigger shutdown logic only once

	mu       sync.Mutex
	hooks    []Hook
	handlers []HandlerHook
}

func NewTracker(runtime *config.Runtime, logger zerolog.Logger) *Tracker {
	t := &Tracker{
		logger:       logger,
		watchSignals: runtime.EnvType != "test",
		logShutdown:  runtime.EnvCloud != "local",
		initiated:    make(chan struct{}),
	}
	t.timingsFromConfig(runtime)

	return t
}

func (t *Tracker) timingsFromConfig(runtime *config.Runtime) config.GracefulShutdownTimings {
	// Setup the config
	var cfg config.GracefulShutdownTimings
	if runtime.GracefulShutdown != nil {
		cfg = *runtime.GracefulShutdown
	}

	// Handle the migration from ShutdownTimout to GracefulShutdown configuration
	if cfg.Total == nil {
		t.gracefulTimeout = runtime.ShutdownTimeout
		if t.gracefulTimeout <= 0 {
			t.gracefulTimeout = 500 * time.Millisecond
		}
	} else {
		t.gracefulTimeout = *cfg.Total
	}
	if t.gracefulTimeout < 0 {
		t.gracefulTimeout = 0
	}

	// Get the handler timeout
	if cfg.Handlers == nil {
		t.handlerTimeout = t.gracefulTimeout - 1*time.Second
	} else {
		t.handlerTimeout = t.gracefulTimeout - *cfg.Handlers
	}
	if t.handlerTimeout < 0 {
		t.handlerTimeout = 0
	}

	// Get the shutdown hook timeout
	if cfg.ShutdownHooks == nil {
		t.shutdownHooksTimeout = t.gracefulTimeout - 1*time.Second
	} else {
		t.shutdownHooksTimeout = t.gracefulTimeout - *cfg.ShutdownHooks
	}
	if t.shutdownHooksTimeout < 0 {
		t.shutdownHooksTimeout = 0
	}

	return cfg
}

// WatchForShutdownSignals watches for shutdown signals (SIGTERM, SIGINT)
// and triggers the graceful shutdown when such a signal is received.
func (t *Tracker) WatchForShutdownSignals() {
	if !t.watchSignals {
		return
	}

	gracefulSignal := make(chan os.Signal, 1)
	signal.Notify(gracefulSignal, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		signalReceived := <-gracefulSignal
		t.Shutdown(signalReceived, nil)
	}()
}

// OnShutdown registers a shutdown handler that will be called when the app
// is gracefully shutting down.
//
// The given context is closed when the graceful shutdown window is closed and it's
// time to forcefully shut down. force.Deadline() can be inspected to learn when this
// will happen in advance.
//
// The shutdown is cooperative: the process will not exit until all shutdown hooks
// have returned, unless the process is forcefully killed by a signal (which may happen
// in certain cloud environments if the graceful shutdown takes longer than its timeout).
//
// If t is nil this function is a no-op.
func (t *Tracker) OnShutdown(fn Hook) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.hooks = append(t.hooks, fn)
}

// RegisterHandlerHook registers a handler hook that will be called when the app
// wants to cancel all currently running handlers.
func (t *Tracker) RegisterHandlerHook(fn HandlerHook) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.handlers = append(t.handlers, fn)
}

// Shutdown triggers the shutdown logic.
// If it has already been triggered, it does nothing and returns immediately.
func (t *Tracker) Shutdown(reasonSignal os.Signal, reasonError error) {
	t.once.Do(func() {
		close(t.initiated)

		// This is so we can debug the shutdown logic, but default it's off
		const debugTraceShutdown = false

		// Create the contexts we need to initiate the shutdown
		gracefulCtx, cancelGraceful := context.WithTimeout(context.Background(), t.gracefulTimeout)
		defer cancelGraceful()
		shutdownCtx, cancelShutdown := context.WithTimeout(gracefulCtx, t.shutdownHooksTimeout)
		defer cancelShutdown()
		handlerCtx, cancelHandler := context.WithTimeout(gracefulCtx, t.handlerTimeout)
		defer cancelHandler()

		if t.logShutdown {
			if reasonSignal != nil {
				t.logger.Warn().Str("signal", reasonSignal.String()).Msg("got shutdown signal, initiating graceful shutdown")
			} else if reasonError != nil {
				t.logger.Err(reasonError).Msg("a fatal error occurred, initiating graceful shutdown")
			} else {
				t.logger.Info().Msg("initiating graceful shutdown")
			}
		}

		// Start a goroutine that will forcefully shutdown the process when the graceful context
		// is closed.
		//
		// If it timed out, we exit with a non-zero exit code to signal that the shutdown was not graceful.
		// If it was cancelled, we exit with a zero exit code to signal that the shutdown was graceful.
		go func() {
			<-gracefulCtx.Done()

			if errors.Is(gracefulCtx.Err(), context.DeadlineExceeded) {
				if t.logShutdown {
					t.logger.Info().Msg("graceful shutdown window closed, forcing shutdown")
				}
				os.Exit(1)
			} else {
				if t.logShutdown {
					t.logger.Info().Msg("graceful shutdown completed")
				}
				os.Exit(0)
			}
		}()

		t.mu.Lock()
		hooks := t.hooks
		handlerHooks := t.handlers
		t.mu.Unlock()

		// Run our hooks concurrently and wait for them to complete.
		var wg sync.WaitGroup
		wg.Add(len(hooks) + len(handlerHooks))

		for _, fn := range hooks {
			fn := fn
			name := functionName(fn)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						t.logger.Err(errs.B().Msg("recovered panic").Err()).Interface("panic", r).Msg("panic encountered during shutdown hook")
					}
				}()

				if debugTraceShutdown {
					defer t.logger.Trace().Str("hook", name).Msg("shutdown hook completed")
					t.logger.Trace().Str("hook", name).Msg("running shutdown hook...")
				}
				fn(shutdownCtx)
			}()
		}

		for _, fn := range handlerHooks {
			fn := fn
			name := functionName(fn)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						t.logger.Err(errs.B().Msg("recovered panic").Err()).Interface("panic", r).Msg("panic encountered running shutdown hook")
					}
				}()

				// Wait for the handler context to be cancelled before calling the handler hook
				<-handlerCtx.Done()
				if debugTraceShutdown {
					defer t.logger.Trace().Str("hook", name).Msg("shutdown hook completed")
					t.logger.Trace().Str("hook", name).Msg("running shutdown hook...")
				}
				fn()
			}()
		}

		wg.Wait()

		// If here we've gracefully shutdown, so we can cancel the graceful context
		cancelGraceful()
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

func functionName(fn any) (rtn string) {
	defer func() {
		if r := recover(); r != nil && rtn == "" {
			rtn = fmt.Sprintf("<panic getting function name: %v>", r)
		}
	}()

	return strings.TrimSuffix(runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name(), "-fm")
}
