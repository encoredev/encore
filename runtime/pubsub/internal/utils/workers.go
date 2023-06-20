package utils

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// Between work processors finishing a work item, how long we debounce before fetching more work
	// (this is to avoid fetching work items in batches of 1)
	workFetchDebounce = 25 * time.Millisecond

	// What is the maximum amount of time we wait before fetching work items when debouncing
	maxFetchDebounce = 250 * time.Millisecond
)

// WorkFetcher is a function that fetches work from a queue, it should fetch at most maxToFetch items
// and block until it has at least one item to return.
type WorkFetcher[Work any] func(ctx context.Context, maxToFetch int) ([]Work, error)

// WorkProcessor is a function that processes a single work item, it should block until the work item is processed
type WorkProcessor[Work any] func(ctx context.Context, work Work) error

// WorkConcurrently fetches work using the given fetch function and then passes it to the worker function
//
// It will fetch at most maxBatchSize items at a time and guarantees that at most maxConcurrency items have been fetched
// and are being processed at any given time.
//
// If maxBatchSize >= 1, will fetch at most maxBatchSize items at a time
// If maxBatchSize <= 0, will fetch as most maxConcurrency items at a time
//
// If maxConcurrency <= 0 then there is no limit on the number of items being processed at a time
//
// This function will block until an error is returned from either the fetcher or the worker functions or until
// the context is cancelled.
func WorkConcurrently[Work any](ctx context.Context, maxConcurrency int, maxBatchSize int, fetch WorkFetcher[Work], worker WorkProcessor[Work]) error {
	if maxConcurrency == 1 {
		// If there's no concurrency, we can just do everything synchronously within this goroutine
		// This avoids the overhead of creating mutexes being used
		return workInSingleRoutine(ctx, fetch, worker)

	} else if maxConcurrency <= 0 {
		// If there's infinite concurrency, we can just do everything by spawning goroutines
		// for each work item
		return workInInfiniteRoutines(ctx, maxBatchSize, fetch, worker)

	} else {
		// Else there's a cap on concurrency, we need to use channels to communicate between the fetcher and the workers
		return workInWorkPool(ctx, maxConcurrency, maxBatchSize, fetch, worker)
	}
}

func workInSingleRoutine[Work any](ctx context.Context, fetch WorkFetcher[Work], worker WorkProcessor[Work]) error {
	for {
		// check if the context has been cancelled before fetching work
		if err := ctx.Err(); err != nil {
			return nil
		}

		// fetch 1 item
		work, err := fetch(ctx, 1)
		if err != nil {
			return err
		}

		// loop over the items (we might get zero, and a buggy implementation might return more than 1, so a loop is safer)
		for _, w := range work {
			// check if the context has been cancelled before processing the work
			if err := ctx.Err(); err != nil {
				return nil
			}

			// process the work
			if err := worker(ctx, w); err != nil {
				return err
			}
		}
	}
}

func workInInfiniteRoutines[Work any](ctx context.Context, maxBatchSize int, fetch WorkFetcher[Work], worker WorkProcessor[Work]) error {
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var firstError error
	var errMutex sync.Mutex
	recordError := func(err error) {
		errMutex.Lock()
		defer errMutex.Unlock()
		if firstError == nil {
			firstError = err
			cancel()
		}
	}

	if maxBatchSize <= 0 {
		maxBatchSize = 100
	}

	for workerCtx.Err() == nil {
		work, err := fetch(workerCtx, maxBatchSize)
		if err != nil {
			recordError(err)
			break
		}

		for _, w := range work {
			w := w
			go func() {
				if err := worker(workerCtx, w); err != nil {
					recordError(err)
				}
			}()
		}
	}

	// Return the first error that was encountered
	errMutex.Lock()
	defer errMutex.Unlock()
	return firstError
}

func workInWorkPool[Work any](ctx context.Context, maxConcurrency int, maxBatchSize int, fetch WorkFetcher[Work], worker WorkProcessor[Work]) error {
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// workChan is a channel that is used to pass work from the fetcher to the workers
	workChan := make(chan Work)

	// workDone is a channel that is used to signal that a worker has finished processing an item
	workDone := make(chan struct{}, maxConcurrency)

	var firstError error
	var errMutex sync.Mutex
	recordError := func(err error) {
		errMutex.Lock()
		defer errMutex.Unlock()
		if firstError == nil {
			firstError = err
			cancel()
		}
	}

	var inFlight atomic.Int64

	// workProcessor is a small wrapper around the worker function that tracks the number of items being processed
	// and cancels the context if an error is returned
	workProcessor := func(work Work) {
		inFlight.Add(1)
		defer inFlight.Add(-1)
		defer func() { workDone <- struct{}{} }()

		if err := worker(workerCtx, work); err != nil {
			recordError(err)
		}
	}

	// fetchProcessor is a small wrapper around the fetcher function that passes the fetched work to the workers
	// it will fetch upto maxConcurrency items at a time in batches of maxBatchSize items
	var lastFetch time.Time
	var debounceTimer *time.Timer
	var fetchLock sync.Mutex
	fetchProcessor := func() {
		// Lock the fetcher so that we don't have multiple fetchers running at the same time
		fetchLock.Lock()
		defer fetchLock.Unlock()
		defer func() { lastFetch = time.Now() }()

		// Work out how many items we need to fetch
		need := maxConcurrency - int(inFlight.Load())

		// Fetch work in batches
		for need > 0 {
			// calculate how many items we need to fetch in this batch
			toFetch := need
			if maxBatchSize > 0 && toFetch > maxBatchSize {
				toFetch = maxBatchSize
			}

			// fetch the work
			work, err := fetch(workerCtx, toFetch)
			if err != nil {
				recordError(err)
				return
			}

			// Pass work to workers
			for _, w := range work {
				workChan <- w
			}

			// Update the number of items we need to fetch
			// if nothing was returned, we will immediately loop and try again
			need -= len(work)
		}
	}

	// Start the workers
	for i := 0; i < maxConcurrency; i++ {
		go func() {
			for {
				select {
				case <-workerCtx.Done():
					return

				case work := <-workChan:
					workProcessor(work)
				}
			}
		}()
	}

	// Add a dummy item to the workDone channel so that the fetcher will be triggered
	// for the first time
	workDone <- struct{}{}

	// Start fetching work
fetchLoop:
	for {
		select {
		case <-workerCtx.Done():
			// If the context is cancelled, we need to stop fetching work
			break fetchLoop

		case <-workDone:
			if debounceTimer != nil {
				debounceTimer.Stop()
				debounceTimer = nil
			}

			if time.Since(lastFetch) > maxFetchDebounce {
				fetchProcessor()
			} else {
				debounceTimer = time.AfterFunc(workFetchDebounce, fetchProcessor)
			}

		}
	}

	// Return the first error that was encountered
	errMutex.Lock()
	defer errMutex.Unlock()
	return firstError
}
