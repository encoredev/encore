package run

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"encr.dev/internal/optracker"
)

type AsyncBuildJobs struct {
	ctx        context.Context
	cancelCtx  context.CancelFunc
	m          sync.Mutex
	wait       sync.WaitGroup
	firstError error
	tracker    *optracker.OpTracker
	start      time.Time
	appID      string
}

func NewAsyncBuildJobs(ctx context.Context, appID string, tracker *optracker.OpTracker) *AsyncBuildJobs {
	ctx, cancelCtx := context.WithCancel(ctx)

	return &AsyncBuildJobs{
		ctx:       ctx,
		cancelCtx: cancelCtx,
		appID:     appID,
		tracker:   tracker,
		start:     time.Now(),
	}
}

func (a *AsyncBuildJobs) Go(description string, track bool, minDuration time.Duration, f func(ctx context.Context) error) {
	a.wait.Add(1)

	trackerID := optracker.NoOperationID
	if track && a.tracker != nil {
		trackerID = a.tracker.Add(description, a.start)
	}

	go func() {
		defer a.wait.Done()

		log.Info().Str("app_id", a.appID).Str("job", description).Msg("starting build job")
		if err := f(a.ctx); err != nil {
			// If the context was canceled, it probably means the error was due to that.
			if a.ctx.Err() != nil {
				if a.tracker != nil {
					a.tracker.Cancel(trackerID)
				}
			} else {
				log.Err(err).Str("app_id", a.appID).Str("job", description).Msg("build job failed")
				if a.tracker != nil {
					a.tracker.Fail(trackerID, err)
				}
				a.recordError(err)
			}
		} else {
			if a.tracker != nil {
				a.tracker.Done(trackerID, minDuration)
			}
			log.Info().Str("app_id", a.appID).Str("job", description).Msg("build job finished")
		}
	}()
}

func (a *AsyncBuildJobs) Wait() error {
	a.wait.Wait()
	return a.firstError
}

func (a *AsyncBuildJobs) recordError(err error) {
	a.m.Lock()
	defer a.m.Unlock()

	a.cancelCtx()

	if a.firstError == nil {
		a.firstError = err
	}
}
