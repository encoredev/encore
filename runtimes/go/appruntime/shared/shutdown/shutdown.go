package shutdown

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/encoreenv"
	"encore.dev/appruntime/shared/health"
	"encore.dev/beta/errs"
	"encore.dev/shutdown"
)

type Handler func(p *Process) error

type Tracker struct {
	logger zerolog.Logger

	watchSignals bool

	timings processTimings

	initiated chan struct{} // closed when graceful shutdown is initiated
	once      sync.Once     // to trigger shutdown logic only once

	mu       sync.Mutex
	handlers []Handler
}

func NewTracker(runtime *config.Runtime, logger zerolog.Logger) *Tracker {
	t := &Tracker{
		watchSignals: runtime.EnvType != "test",
		initiated:    make(chan struct{}),
		timings:      timingsFromConfig(runtime),
	}

	// Determine the appropriate log level.
	switch {
	case encoreenv.Get("ENCORE_SHUTDOWN_TRACE") != "":
		t.logger = logger.Level(zerolog.TraceLevel)
	case runtime.EnvCloud == "local":
		// For local development only show errors
		t.logger = logger.Level(zerolog.ErrorLevel)
	default:
		t.logger = logger.Level(zerolog.InfoLevel)
	}

	return t
}

type processTimings struct {
	// keepAcceptingFor is the duration from the moment we receive a SIGTERM
	// after which we stop accepting new requests. However we will will
	// report being unhealthy to the load balancer immediately.
	//
	// This is needed as in a Kubernetes environment, the pod sent a SIGTERM
	// once it's replacement is ready, however it will take some time for that
	// to propagate to the load balancer. If we stop accepting requests immediately
	// we will have a period of time where the load balancer will still send
	// requests to the pod, which will be rejected. This will cause the load
	// balancer to report 502 errors.
	//
	// See: https://cloud.google.com/kubernetes-engine/docs/how-to/container-native-load-balancing#traffic_does_not_reach_endpoints
	keepAcceptingFor time.Duration

	// cancelRunningTasksAfter is the duration (measured from shutdown initiation)
	// after which running tasks (outstanding API calls & PubSub messages) have
	// their contexts canceled.
	cancelRunningTasksAfter time.Duration

	// forceCloseTasksGrace is the duration (measured from when canceling running tasks)
	// after which the tasks are considered done, even if they're still running.
	forceCloseTasksGrace time.Duration

	// forceShutdownAfter is the duration (measured from shutdown initiation)
	// after which the shutdown process enters the "force shutdown" phase,
	// tearing down infrastructure resources.
	forceShutdownAfter time.Duration

	// forceShutdownGrace is the grace period after beginning the force shutdown
	// before the shutdown is marked as completed, causing the process to exit.
	forceShutdownGrace time.Duration
}

func timingsFromConfig(runtime *config.Runtime) processTimings {
	// Setup the config
	var cfg config.GracefulShutdownTimings
	if runtime.GracefulShutdown != nil {
		cfg = *runtime.GracefulShutdown
	}

	t := processTimings{
		keepAcceptingFor:     0,
		forceCloseTasksGrace: 1 * time.Second,
		forceShutdownGrace:   1 * time.Second,
	}

	// Handle the migration from ShutdownTimout to GracefulShutdown configuration
	var totalTime time.Duration
	if cfg.Total == nil {
		t.forceShutdownAfter = runtime.ShutdownTimeout
		if t.forceShutdownAfter <= 0 {
			t.forceShutdownAfter = 5 * time.Second
		}
		totalTime = runtime.ShutdownTimeout
	} else {
		t.forceShutdownAfter = *cfg.Total - t.forceShutdownGrace
		if t.forceShutdownAfter <= 0 {
			t.forceShutdownAfter = 500 * time.Millisecond
		}
		totalTime = *cfg.Total
	}

	// Get the handler timeout
	if cfg.Handlers == nil {
		t.cancelRunningTasksAfter = t.forceShutdownAfter - t.forceCloseTasksGrace
	} else {
		t.cancelRunningTasksAfter = *cfg.Handlers
	}
	if t.cancelRunningTasksAfter < 0 {
		t.cancelRunningTasksAfter = 0
	}

	k8sGraceTimeSecs := encoreenv.Get("ENCORE_K8S_GRACE_TERMINATION_SECONDS")
	if k8sGraceTimeSecs != "" {
		if graceSecs, err := strconv.Atoi(k8sGraceTimeSecs); err != nil {
			panic(fmt.Sprintf("invalid value for ENCORE_K8S_GRACE_TERMINATION_SECONDS (sepected an interger): %s", err))
		} else {
			// If we know what the grace termination is for the kubernetes pods, we want to keep accepting new traffic
			// for almost all of that duration - minus what the Encore runtime needs to perform a graceful shutdown.
			//
			// We'll immediately report a health failure when SIGTERM is received, however we'll still accept new
			// traffic as we wait for routers and load balancers to update have propergated that we're trying
			// to cleanly shutdown.
			t.keepAcceptingFor = (time.Duration(graceSecs) * time.Second) - totalTime
			if t.keepAcceptingFor < 0 {
				t.keepAcceptingFor = 0
			}
		}
	}

	return t
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

// RegisterShutdownHandler registers a shutdown handler that will be called when the app
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
func (t *Tracker) RegisterShutdownHandler(fn Handler) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.handlers = append(t.handlers, fn)
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

// HealthCheck returns an health check failure once a SIGTERM has been received.
//
// This is to allow load balancers to detect this instance is shutting down
// and should not be routed to for new traffic.
func (t *Tracker) HealthCheck(_ context.Context) []health.CheckResult {
	var reportError error
	if t.ShutdownInitiated() {
		reportError = errors.New("SIGTERM has been received, graceful shutdown started")
	}

	return []health.CheckResult{{
		Name: "shutdown-signal-monitoring",
		Err:  reportError,
	}}
}

// Shutdown triggers the shutdown logic.
// If it has already been triggered, it does nothing and returns immediately.
func (t *Tracker) Shutdown(reasonSignal os.Signal, reasonError error) {
	t.once.Do(func() {
		close(t.initiated)

		if reasonError != nil {
			t.logger.Err(reasonError).Msg("a fatal error occurred, initiating graceful shutdown")
		} else if reasonSignal != nil {
			t.logger.Info().Str("signal", reasonSignal.String()).Msg("got shutdown signal, initiating graceful shutdown")
		} else {
			t.logger.Trace().Msg("initiating graceful shutdown")
		}

		// If we received a SIGTERM and have a configured keepAcceptingFor duration
		// then log the fact we're going to continue accepting new requests and then
		// sleep for that time before begining to graceful shutdown.
		if reasonSignal == syscall.SIGTERM && t.timings.keepAcceptingFor > 0 {
			t.logger.Info().Str("duration", t.timings.keepAcceptingFor.String()).Msg("continuing to accept requests for a short period of time to allow the load balancer to update")
			time.Sleep(t.timings.keepAcceptingFor)
			t.logger.Info().Msg("stopping to accept new requests and continuing graceful shutdown")
		}

		p := t.beginShutdownProcess()
		go t.runShutdownHandlers(p)
		t.exitOnCompletion(p)
	})
}

// Process mirrors [encore.dev/shutdown.Progress] but has internal fields
// for controlling the behavior.
type Process struct {
	Log                       *zerolog.Logger
	OutstandingRequests       context.Context
	cancelOutstandingRequests context.CancelFunc

	OutstandingPubSubMessages       context.Context
	cancelOutstandingPubSubMessages context.CancelFunc

	OutstandingTasks       context.Context
	cancelOutstandingTasks context.CancelFunc

	ForceCloseTasks       context.Context
	cancelForceCloseTasks context.CancelFunc

	ServicesShutdownCompleted     context.Context
	markServicesShutdownCompleted context.CancelCauseFunc

	ForceShutdown       context.Context
	cancelForceShutdown context.CancelFunc

	handlersCompleted     context.Context
	markHandlersCompleted context.CancelCauseFunc

	// ShutdownCompleted is closed when all shutdown hooks have returned.
	ShutdownCompleted     context.Context
	markShutdownCompleted context.CancelCauseFunc
}

// cleanShutdown is a sentinel error used by the shutdown logic to indicate
// a clean shutdown, via context.Cause.
var cleanShutdown = errors.New("clean shutdown")

func (t *Tracker) beginShutdownProcess() *Process {
	start := time.Now()

	tt := t.timings
	outstandingTasks, cancelOutstandingTasks := context.WithDeadline(context.Background(), start.Add(tt.cancelRunningTasksAfter+tt.forceCloseTasksGrace))
	outstandingRequests, cancelOutstandingRequests := context.WithCancel(outstandingTasks)
	outstandingPubSubMessages, cancelOutstandingPubSubMessages := context.WithCancel(outstandingTasks)

	forceCloseTasks, cancelForceCloseTasks := context.WithDeadline(outstandingTasks, start.Add(tt.cancelRunningTasksAfter))

	forceShutdown, cancelForceShutdown := context.WithDeadline(context.Background(), start.Add(tt.forceShutdownAfter))

	serviceShutdownCompleted, cancelServiceShutdownCompleted := context.WithCancelCause(context.Background())
	handlersCompleted, cancelHandlersCompleted := context.WithCancelCause(context.Background())

	shutdownCompleted, cancelShutdownCompleted := context.WithCancelCause(context.Background())

	// Close the runningHandlers context when both
	// outstandingRequests and outstandingPubSubMessages are done.
	go func() {
		<-outstandingRequests.Done()
		<-outstandingPubSubMessages.Done()
		cancelOutstandingTasks()

		// This is redundant (the context is derived from runningTasks),
		// but it makes the linter happy.
		cancelForceCloseTasks()
	}()

	// Cancel forceShutdown early if running tasks and handlers complete.
	go func() {
		<-outstandingTasks.Done()
		<-handlersCompleted.Done()
		cancelForceShutdown()
	}()

	// Mark the shutdown completed.
	go func() {
		<-forceShutdown.Done()
		// When forceShutdown is done, see if it was due to reaching the deadline (unclean shutdown)
		// or if we canceled the context early (clean shutdown).
		if errors.Is(forceShutdown.Err(), context.Canceled) {
			cancelShutdownCompleted(cleanShutdown)
			return
		} else {
			// We reached the deadline. The ForceShutdown context was canceled just now,
			// so give it another second to let the shutdown handlers finish.
			timeout, cancel := context.WithTimeout(handlersCompleted, tt.forceShutdownGrace)
			defer cancel()
			<-timeout.Done()

			if errors.Is(timeout.Err(), context.Canceled) {
				// The handlers did eventually complete, so this is a clean shutdown.
				cancelShutdownCompleted(cleanShutdown)
			} else {
				cancelShutdownCompleted(timeout.Err())
			}
		}
	}()

	return &Process{
		Log:                       &t.logger,
		OutstandingRequests:       outstandingRequests,
		cancelOutstandingRequests: cancelOutstandingRequests,

		OutstandingPubSubMessages:       outstandingPubSubMessages,
		cancelOutstandingPubSubMessages: cancelOutstandingPubSubMessages,

		OutstandingTasks:       outstandingTasks,
		cancelOutstandingTasks: cancelOutstandingTasks,

		ForceCloseTasks:       forceCloseTasks,
		cancelForceCloseTasks: cancelForceCloseTasks,

		ForceShutdown:       forceShutdown,
		cancelForceShutdown: cancelForceShutdown,

		ServicesShutdownCompleted:     serviceShutdownCompleted,
		markServicesShutdownCompleted: cancelServiceShutdownCompleted,

		handlersCompleted:     handlersCompleted,
		markHandlersCompleted: cancelHandlersCompleted,

		ShutdownCompleted:     shutdownCompleted,
		markShutdownCompleted: cancelShutdownCompleted,
	}
}

// Progress converts p into the public shutdown.Progress type.
func (p *Process) Progress() shutdown.Progress {
	return shutdown.Progress{
		OutstandingRequests:       p.OutstandingRequests,
		OutstandingPubSubMessages: p.OutstandingPubSubMessages,
		OutstandingTasks:          p.OutstandingTasks,
		ForceCloseTasks:           p.ForceCloseTasks,
		ForceShutdown:             p.ForceShutdown,
	}
}

func (p *Process) MarkOutstandingRequestsCompleted() {
	p.cancelOutstandingRequests()
}

func (p *Process) MarkOutstandingPubSubMessagesCompleted() {
	p.cancelOutstandingPubSubMessages()
}

func (p *Process) MarkServicesShutdownCompleted(err error) {
	// TODO change error type to capture where the service came from
	if err != nil {
		p.markServicesShutdownCompleted(err)
	} else {
		p.markServicesShutdownCompleted(cleanShutdown)
	}
}

// WasCleanShutdown reports whether the shutdown was clean.
// Its return value is undefined before p.shutdownCompleted is closed.
func (p *Process) WasCleanShutdown() bool {
	return errors.Is(context.Cause(p.ShutdownCompleted), cleanShutdown)
}

type shutdownError struct {
	handlerName string
	err         error
}

func (e shutdownError) Error() string {
	return fmt.Sprintf("shutdown handler %q: %v", e.handlerName, e.err)
}

func (e shutdownError) Unwrap() error {
	return e.err
}

type shutdownErrors struct {
	errors []error
}

func (e shutdownErrors) Unwrap() []error {
	return e.errors
}

func (e shutdownErrors) Error() string {
	switch len(e.errors) {
	case 0:
		return "no shutdown errors"
	case 1:
		return e.errors[0].Error()
	default:
		var buf strings.Builder
		buf.WriteString("multiple shutdown errors: ")
		for i, err := range e.errors {
			if i > 0 {
				buf.WriteString("; ")
			}
			buf.WriteString(err.Error())
		}
		return buf.String()
	}
}

// runShutdownHandlers runs the registered shutdown handlers.
func (t *Tracker) runShutdownHandlers(p *Process) {
	var (
		shutdownErrorMu sync.Mutex
		shutdownErrs    []error
	)
	addShutdownErr := func(err shutdownError) {
		shutdownErrorMu.Lock()
		defer shutdownErrorMu.Unlock()
		shutdownErrs = append(shutdownErrs, err)
	}

	// Mark the handlers as completed when we're done.
	defer func() {
		shutdownErrorMu.Lock()
		errList := shutdownErrs
		shutdownErrorMu.Unlock()

		// Determine the error to use.
		var shutdownErr error
		if len(errList) > 0 {
			shutdownErr = shutdownErrors{errors: errList}
		}

		t.logger.Trace().Err(shutdownErr).Msg("all shutdown hooks completed")

		if shutdownErr != nil {
			p.markHandlersCompleted(shutdownErr)
		} else {
			p.markHandlersCompleted(cleanShutdown)
		}
	}()

	t.mu.Lock()
	handlers := t.handlers
	t.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(len(handlers))

	for _, fn := range handlers {
		fn := fn
		name := functionName(fn)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					err := shutdownError{
						handlerName: name,
						err:         errs.B().Msgf("panic: %s", r).Err(),
					}
					addShutdownErr(err)
					t.logger.Err(err).Interface("panic", r).Msg("panic encountered during shutdown hook")
				}
			}()

			defer t.logger.Trace().Str("hook", name).Msg("shutdown hook completed")
			t.logger.Trace().Str("hook", name).Msg("running shutdown hook...")
			if err := fn(p); err != nil {
				shutdownErr := shutdownError{handlerName: name, err: err}
				t.logger.Error().Err(shutdownErr).Str("hook", name).Msg("shutdown handler returned an error")
				addShutdownErr(shutdownErr)
			}
		}()
	}

	wg.Wait()
}

// exitOnCompletion exits the process when the shutdown is completed.
func (t *Tracker) exitOnCompletion(p *Process) {
	<-p.ShutdownCompleted.Done()

	if p.WasCleanShutdown() {
		t.logger.Trace().Msg("graceful shutdown completed")
		os.Exit(0)
	} else {
		t.logger.Trace().Msg("graceful shutdown window closed, forcing shutdown")
		os.Exit(1)
	}
}
