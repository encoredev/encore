package utils

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"encore.dev/beta/errs"
)

const (
	// Between work processors finishing a work item, how long we debounce before fetching more work
	// (this is to avoid fetching work items in batches of 1)
	workFetchDebounce = 25 * time.Millisecond

	// What is the maximum amount of time we wait before fetching work items when debouncing
	maxFetchDebounce = 250 * time.Millisecond

	noWorkDebounce = 500 * time.Millisecond
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
//
// In the event of an error occurring, calls to worker will be allowed to continue in the background until the
// context is cancelled however, this function will still return immediately with the error. Thus if you immediately call
// this again you could end up with 2x maxConcurrency workers running at the same time. (1x from the original run who
// are still processing work and 1x from the new run).
func WorkConcurrently[Work any](ctxs *Contexts, maxConcurrency int, maxBatchSize int, fetch WorkFetcher[Work], worker WorkProcessor[Work]) error {
	fetchWithPanicHandling := func(ctx context.Context, maxToFetch int) (work []Work, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = errs.B().Msgf("panic: %v", r).Err()
			}
		}()
		return fetch(ctx, maxToFetch)
	}

	workWithPanicHandling := func(ctx context.Context, work Work) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = errs.B().Msgf("panic: %v", r).Err()
			}
		}()
		return worker(ctx, work)
	}

	if maxConcurrency == 1 {
		// If there's no concurrency, we can just do everything synchronously within this goroutine
		// This avoids the overhead of creating mutexes being used
		return workInSingleRoutine(ctxs, fetchWithPanicHandling, workWithPanicHandling)

	} else if maxConcurrency <= 0 {
		// If there's infinite concurrency, we can just do everything by spawning goroutines
		// for each work item
		return workInInfiniteRoutines(ctxs, maxBatchSize, fetchWithPanicHandling, workWithPanicHandling)

	} else {
		// Else there's a cap on concurrency, we need to use channels to communicate between the fetcher and the workers
		return workInWorkPool(ctxs, maxConcurrency, maxBatchSize, fetchWithPanicHandling, workWithPanicHandling)
	}
}

func workInSingleRoutine[Work any](ctxs *Contexts, fetch func(ctx context.Context, maxToFetch int) (work []Work, err error), worker func(ctx context.Context, work Work) (err error)) error {
	for {
		// check if the context has been cancelled before fetching work
		if err := ctxs.Fetch.Err(); err != nil {
			return nil
		}

		// fetch 1 item
		work, err := fetch(ctxs.Fetch, 1)
		if err != nil {
			return err
		}

		// loop over the items (we might get zero, and a buggy implementation might return more than 1, so a loop is safer)
		for _, w := range work {
			// check if the context has been cancelled before processing the work
			if err := ctxs.Handler.Err(); err != nil {
				return nil
			}

			// process the work
			if err := worker(ctxs.Handler, w); err != nil {
				return err
			}
		}
	}
}

func workInInfiniteRoutines[Work any](ctxs *Contexts, maxBatchSize int, fetch func(ctx context.Context, maxToFetch int) (work []Work, err error), worker func(ctx context.Context, work Work) (err error)) error {
	fetchCtx, cancel := context.WithCancelCause(ctxs.Fetch)
	defer cancel(nil)

	if maxBatchSize <= 0 {
		maxBatchSize = 100
	}

	for fetchCtx.Err() == nil {
		work, err := fetch(fetchCtx, maxBatchSize)
		if err != nil {
			cancel(err)
			break
		}

		for _, w := range work {
			w := w
			go func() {
				// the worker uses the parent context, such that if we have a fetch error, the existing workers will
				// continue to run until they finish processing their work
				if err := worker(ctxs.Handler, w); err != nil {
					cancel(err)
				}
			}()
		}
	}

	// Return the reason for cancellation if it wasn't due to the parent context being cancelled
	cancelCause := context.Cause(fetchCtx)
	if errors.Is(cancelCause, context.Canceled) {
		return nil
	}
	return cancelCause
}

func workInWorkPool[Work any](ctxs *Contexts, maxConcurrency int, maxBatchSize int, doFetch func(ctx context.Context, maxToFetch int) (work []Work, err error), doProcessWork func(ctx context.Context, work Work) (err error)) error {
	fetchCtx, cancelFetch := context.WithCancelCause(ctxs.Fetch)
	defer cancelFetch(nil)

	// workChan is a channel that is used to pass work from the fetcher to the workers
	workChan := make(chan Work)
	defer close(workChan) // close the channel when we're done so the workers know to stop

	numWorkers := maxConcurrency

	// workDone is a channel that is used to signal that a worker has finished processing an item
	workDone := make(chan struct{})

	var workItemsBeingProcessed atomic.Int64

	// processWorkItem is a small wrapper around the worker function that tracks the number of items being processed
	// and cancels the context if an error is returned
	processWorkItem := func(work Work) {
		workItemsBeingProcessed.Add(1)

		defer func() {
			// Decrement the number of items being processed.
			// Note: it's important this happens BEFORE we
			// try to unblock the fetcher, otherwise we can end up
			// with a race where we unblock the fetcher but it doesn't
			// find any work to do and deadlocks.
			workItemsBeingProcessed.Add(-1)

			// Attempt to unblock the fetcher if they're
			// waiting for a work item to complete.
			select {
			case workDone <- struct{}{}:
			default:
			}
		}()

		// We use the parent context here, such that if we have a fetch error, the existing workers will
		// continue to run until they finish processing any work already have started on
		if err := doProcessWork(ctxs.Handler, work); err != nil {
			cancelFetch(err)
		}
	}
	worker := func() {
		for work := range workChan {
			processWorkItem(work)
		}
	}

	// Start the workers
	for i := 0; i < numWorkers; i++ {
		go worker()
	}

	// spuriousWakeup is a ticker used to periodically wake the fetcher
	// even in the absence of any work being completed. This is here
	// because there's a (theoretical) race condition in the fetcher logic
	// in cases where all the workers are busy:
	//
	//    1. The fetcher detects all workers are busy
	//    2. Before getting to the select {} statement, all workers complete
	//       their work, and fail to send on the workDone channel because
	//       the fetcher isn't yet waiting on it.
	//    3. The fetcher gets stuck waiting indefinitely for a work item to complete,
	//       but there is no work being done.
	//
	// Guard against this by having a spurious periodic wakeup. In normal circumstances
	// this won't ever be used.
	spuriousWakeup := time.NewTicker(1 * time.Second)
	defer spuriousWakeup.Stop()

	// Start fetching work
FetchLoop:
	for fetchCtx.Err() == nil {
		// Determine how many items to fetch
		toFetch := maxConcurrency - int(workItemsBeingProcessed.Load())
		if maxBatchSize > 0 && toFetch > maxBatchSize {
			toFetch = maxBatchSize
		}

		if toFetch == 0 {
			// All our workers are busy, so we can't fetch any more work right now.
			// Wait until there's more work.
			select {
			case <-fetchCtx.Done():
				// We're done; stop fetching.
				break FetchLoop
			case <-workDone:
				// A work item has been completed. Wait for a little bit
				// before we retry the loop to "debounce" and allow for a few
				// more work items to complete so we can fetch bigger batches.
				time.Sleep(workFetchDebounce)
				continue FetchLoop
			case <-spuriousWakeup.C:
				// See comment on spuriousWakeup above for motivation.
				continue FetchLoop
			}
		}

		// We have some work to fetch.
		work, err := doFetch(fetchCtx, toFetch)
		if err != nil {
			cancelFetch(err)
			break FetchLoop
		}

		// If we didn't get any items, sleep before we try again
		// to avoid hammering the server.
		if len(work) == 0 {
			time.Sleep(noWorkDebounce)
			continue FetchLoop
		}

		// Pass the work to workers
		for _, w := range work {
			select {
			case workChan <- w:
				// success, we passed the work to a worker
			case <-fetchCtx.Done():
				break FetchLoop
			}
		}
	}

	// Return the reason for cancellation, unless it was due to the parent context being cancelled.
	cause := context.Cause(fetchCtx)
	if errors.Is(cause, context.Canceled) {
		cause = nil
	}
	return cause
}
