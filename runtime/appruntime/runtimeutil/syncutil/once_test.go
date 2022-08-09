package syncutil

import (
	"errors"
	"testing"
)

func TestOnce(t *testing.T) {
	timesRan := 0
	f := func() error {
		timesRan++
		return nil
	}

	once := Once{}

	const numCalls = 10
	results := make(chan error, numCalls)

	for i := 0; i < numCalls; i++ {
		go func() {
			results <- once.Do(f)
		}()
	}

	for i := 0; i < numCalls; i++ {
		if err := <-results; err != nil {
			t.Errorf("Expected no errors, got %v", err)
		}
	}

	if timesRan != 1 {
		t.Errorf("Expected to run one time, ran %d", timesRan)
	}
}

// TestOnceErroring verifies we retry on every error, but stop after
// the first success.
func TestOnceErroring(t *testing.T) {
	timesRan := 0
	f := func() error {
		timesRan++
		if timesRan < 3 {
			return errors.New("retry")
		}
		return nil
	}

	once := Once{}
	const numCalls = 10
	results := make(chan error, numCalls)

	for i := 0; i < numCalls; i++ {
		go func() {
			results <- once.Do(f)
		}()
	}

	numErrs := 0
	for i := 0; i < numCalls; i++ {
		if err := <-results; err != nil {
			numErrs++
		}
	}

	if numErrs != 2 {
		t.Errorf("Expected two errors, got %d", numErrs)
	}

	if timesRan != 3 {
		t.Errorf("Expected to run two times, ran %d", timesRan)
	}
}
