package run

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"encr.dev/internal/optracker"
)

type asyncBuildJobs struct {
	ctx        context.Context
	cancelCtx  context.CancelFunc
	m          sync.Mutex
	wait       sync.WaitGroup
	firstError error
	tracker    *optracker.OpTracker
	start      time.Time
	appID      string
}

func newAsyncBuildJobs(ctx context.Context, appID string, tracker *optracker.OpTracker) *asyncBuildJobs {
	ctx, cancelCtx := context.WithCancel(ctx)

	return &asyncBuildJobs{
		ctx:       ctx,
		cancelCtx: cancelCtx,
		appID:     appID,
		tracker:   tracker,
		start:     time.Now(),
	}
}

func (a *asyncBuildJobs) Go(description string, track bool, minDuration time.Duration, f func(ctx context.Context) error) {
	a.wait.Add(1)

	trackerID := optracker.NoOperationID
	if track {
		trackerID = a.tracker.Add(description, a.start)
	}

	go func() {
		defer a.wait.Done()

		log.Info().Str("app_id", a.appID).Str("job", description).Msg("starting build job")
		if err := f(a.ctx); err != nil {
			log.Err(err).Str("app_id", a.appID).Str("job", description).Msg("build job failed")
			a.tracker.Fail(trackerID, err)
			a.recordError(err)
		} else {
			a.tracker.Done(trackerID, minDuration)
			log.Info().Str("app_id", a.appID).Str("job", description).Msg("build job finished")
		}
	}()
}

func (a *asyncBuildJobs) Wait() error {
	a.wait.Wait()
	return a.firstError
}

func (a *asyncBuildJobs) recordError(err error) {
	a.m.Lock()
	defer a.m.Unlock()

	a.cancelCtx()

	if a.firstError == nil {
		a.firstError = err
	}
}
