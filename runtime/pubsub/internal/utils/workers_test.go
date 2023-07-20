package utils

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestWorkConcurrently(t *testing.T) {
	t.Parallel()

	params := []struct {
		concurrency  int
		maxBatchSize int
		fetchErr     error
		processErr   error
	}{
		// Simple concurrency tests
		{1, 10, nil, nil},
		{10, 10, nil, nil},
		{50, 50, nil, nil},

		// Test batch sizes
		{50, 0, nil, nil}, // Unlimited batch size
		{50, 1, nil, nil},
		{50, 10, nil, nil},

		// Unlimited concurrency
		{-1, 0, nil, nil},  // No batch size and unlimited concurrency
		{-1, 10, nil, nil}, // Unlimited concurrency, but a batch size

		// Test errors
		{50, 10, fmt.Errorf("fetch error"), nil},
		{50, 10, nil, fmt.Errorf("process error")},
	}

	for _, tt := range params {
		tt := tt
		t.Run(fmt.Sprintf("c%d_b%d", tt.concurrency, tt.maxBatchSize), func(t *testing.T) {
			t.Parallel()
			c := qt.New(t)

			// Create a context which will timeout the test
			timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer timeoutCancel()

			// Then create a context which will cancel the work generator off that
			// (which we use to break out of the work generator loop)
			ctx, cancel := context.WithCancel(timeoutCtx)
			defer cancel()

			// The number of items we've generated
			toGenerate := tt.concurrency * 3 // We want to generate enough work to fill the workers plus test other outcomes
			if toGenerate <= 0 {
				toGenerate = 2_000 // If we have unlimited concurrency, we need to generate a lot of work
			}
			toReturnFromFetcher := make([]int, toGenerate)
			for i := 0; i < toGenerate; i++ {
				toReturnFromFetcher[i] = i
			}
			type fetchReq struct {
				toFetch     int
				numReturned int
			}
			var fetchRequests []*fetchReq // We want to test that the fetcher is called the correct number of times

			// We want to test that the concurrency is respected and the max concurrency is reached
			// so this section setups a counter and max counter to track the number of active workers
			var counterMu sync.Mutex
			var activeWorkers int
			var maxActiveWorkers int
			incActiveWorkers := func() {
				counterMu.Lock()
				defer counterMu.Unlock()
				activeWorkers++
				if activeWorkers > maxActiveWorkers {
					maxActiveWorkers = activeWorkers
				}
			}
			decActiveWorkers := func() {
				counterMu.Lock()
				defer counterMu.Unlock()
				activeWorkers--
			}

			// We want to test that all the work was received by the processor
			// so we'll track the work received in a slice protected by a mutex
			var workMu sync.Mutex
			var receivedWork []int

			// Create the fetcher function
			fetcher := func(ctx context.Context, toFetch int) ([]int, error) {
				// No work to fetch return nothing
				if len(toReturnFromFetcher) == 0 {
					return nil, nil
				}

				// record this fetch request
				req := &fetchReq{
					toFetch: toFetch,
				}
				fetchRequests = append(fetchRequests, req)

				// One of the fetches should return no data
				switch len(fetchRequests) {
				case 2:
					// simulate only one piece of data on fetch 1
					toFetch = 1
				case 3:
					// simulate no data on fetch 2
					time.Sleep(500 * time.Millisecond)
					return nil, nil
				case 4:
					// simulate only half the data is available on fetch 3
					toFetch = toFetch / 2
				case 5:
					// If we have a fetch error, return it on fetch 4
					if tt.fetchErr != nil {
						return nil, tt.fetchErr
					}
				}

				if toFetch > len(toReturnFromFetcher) {
					toFetch = len(toReturnFromFetcher)
				}
				rtn := make([]int, toFetch)
				copy(rtn, toReturnFromFetcher[0:toFetch])
				toReturnFromFetcher = toReturnFromFetcher[toFetch:]

				// If we've generated enough work to fill the workers, cancel the context in a little bit
				// giving time for the workers to process the work
				if len(toReturnFromFetcher) == 0 {
					go func() {
						// Wait a little bit to give the workers time to process the work
						// before cancelling the context as on a slow machine, the workers
						// might not have started processing the work yet
						time.Sleep(5 * time.Second)
						cancel()
					}()
				}

				req.numReturned = len(rtn)
				return rtn, nil
			}

			// Create the processor function
			processor := func(ctx context.Context, work int) error {
				incActiveWorkers()
				defer decActiveWorkers()

				// simulate some work
				time.Sleep(10 * time.Millisecond)

				workMu.Lock()
				defer workMu.Unlock()
				receivedWork = append(receivedWork, work)

				// If we have a process error, return it around half way through the work
				if tt.processErr != nil && len(receivedWork) > (toGenerate/2) {
					return tt.processErr
				}

				if len(receivedWork) == toGenerate {
					// If we've received all the work, cancel the context
					cancel()
				}

				return nil
			}

			err := WorkConcurrently(NewContexts(ctx), tt.concurrency, tt.maxBatchSize, fetcher, processor)

			workMu.Lock()
			defer workMu.Unlock()

			// Run assertions on the exit conditions
			c.Assert(timeoutCtx.Err(), qt.IsNil, qt.Commentf("test timed out - not all work was fetched within the timeout"))
			switch {
			case tt.fetchErr != nil:
				c.Assert(err, qt.ErrorIs, tt.fetchErr, qt.Commentf("unexpected error from work concurrently"))
				c.Assert(len(receivedWork) < toGenerate, qt.IsTrue, qt.Commentf("all the work was fetched even though there was a fetch error"))
				return
			case tt.processErr != nil:
				c.Assert(err, qt.ErrorIs, tt.processErr, qt.Commentf("unexpected error from work concurrently"))
				c.Assert(len(receivedWork) < toGenerate, qt.IsTrue, qt.Commentf("all the work was fetched even though there was a process error"))
				return
			default:
				c.Assert(err, qt.IsNil, qt.Commentf("unexpected error from work concurrently"))
			}

			// Run assertions on the processed data
			c.Assert(receivedWork, qt.HasLen, toGenerate, qt.Commentf("not all work was received/processed"))
			if tt.concurrency > 0 {
				c.Assert(maxActiveWorkers <= tt.concurrency, qt.IsTrue, qt.Commentf("max concurrency was not respected; reached %d workers", maxActiveWorkers))
				c.Assert(maxActiveWorkers == tt.concurrency, qt.IsTrue, qt.Commentf("max concurrency was not reached; only got %d workers at one time", maxActiveWorkers))
			}
			sort.Ints(receivedWork)
			for i, work := range receivedWork {
				c.Assert(work, qt.Equals, i, qt.Commentf("unexpected work received (once sorted); expected %d, got %d", i, work))
			}

			// Run assertions on the fetch requests
			maxBatchSize := tt.maxBatchSize
			if maxBatchSize <= 0 || (maxBatchSize > tt.concurrency && tt.concurrency > 0) {
				if tt.concurrency > 0 {
					maxBatchSize = tt.concurrency
				} else {
					maxBatchSize = 100
				}
			}
			c.Assert(fetchRequests[0].toFetch, qt.Equals, maxBatchSize, qt.Commentf("first fetch request was not the max batch size"))
			c.Assert(fetchRequests[0].numReturned, qt.Equals, maxBatchSize, qt.Commentf("first fetch request did not return a full batch"))
			for i, req := range fetchRequests {
				c.Assert(req.toFetch, qt.Not(qt.Equals), 0, qt.Commentf("fetch request %d was 0", i))
				c.Assert(req.toFetch <= maxBatchSize, qt.IsTrue, qt.Commentf("max batch size was not respected; requested %d items on fetch %d", req, i))
				c.Assert(req.toFetch >= req.numReturned, qt.IsTrue, qt.Commentf("test function returned too many items"))
			}
		})
	}
}

func TestWorkConcurrentlyLoad(t *testing.T) {
	t.Skip()

	const load = 20_000
	msg := make([]string, load)
	for i := 0; i < load; i++ {
		msg[i] = fmt.Sprintf("msg %d", i)
	}

	var (
		mu  sync.Mutex
		idx int

		wMu sync.Mutex
		wrk []string
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var err error

	for ctx.Err() == nil {
		err = WorkConcurrently(NewContexts(ctx), 25, 10, func(ctx context.Context, maxToFetch int) ([]string, error) {
			mu.Lock()
			defer mu.Unlock()

			if idx >= load {
				return nil, nil
			}

			toFetch := maxToFetch
			if toFetch > load-idx {
				toFetch = load - idx
			}
			if toFetch == 0 {
				return nil, nil
			}

			rtn := make([]string, toFetch)
			copy(rtn, msg[idx:idx+toFetch])
			idx += toFetch

			return rtn, nil
		}, func(ctx context.Context, work string) error {
			time.Sleep(100 * time.Millisecond)
			wMu.Lock()
			defer wMu.Unlock()

			wrk = append(wrk, work)

			if len(wrk)%250 == 0 {
				panic("too much work")
			}

			if len(wrk) == load {
				cancel()
			}
			return nil
		})

		fmt.Printf("err (worked %d/%d - sent %d): %v\n", len(wrk), load, idx, err)
	}

	if err != nil {
		t.Fatalf("err (worked %d/%d - sent %d): %v", len(wrk), load, idx, err)
	}

	if len(wrk) != load {
		t.Fatalf("expected %d work items, got %d", load, len(wrk))
	}
}
