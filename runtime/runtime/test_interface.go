package runtime

import (
	"context"
	"runtime/debug"
	"testing"
	"time"
	_ "unsafe"

	"encore.dev/runtime/trace"
)

// encoreTestStart is called when a test starts running. This allows Encore's testing framework to
// isolate behaviour between different tests on global state.
//go:linkname encoreTestStart testing.encoreTestStart
func encoreTestStart(t *testing.T) {
	var parent *Request
	if g := encoreGetG(); g != nil && g.req != nil {
		parent = g.req.data
		encoreClearReq()
	}

	spanID, err := trace.GenSpanID()
	if err != nil {
		t.Fatalf("encoreTestStart: failed to generate span ID: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	logger := Logger().With().Str("test", t.Name()).Logger()
	req := &Request{
		Type:   Test,
		SpanID: spanID,
		Start:  time.Now(),
		Traced: false,
		Test: &TestData{
			Ctx:     ctx,
			Cancel:  cancel,
			Current: t,
			Parent:  parent,
		},
		Logger: &logger,
	}
	encoreBeginReq(spanID, req, false)
}

// encoreTestPaused is called when a test is paused. This allows Encore's testing framework to
// isolate behaviour between different tests on global state.
//go:linkname encoreTestPaused testing.encoreTestPaused
func encoreTestPaused(t *testing.T) {
}

// encoreTestResumed is called when a test is resumed after being paused. This allows Encore's testing framework to clear down any state from the test
// and to perform any assertions on that state that it needs to.
//go:linkname encoreTestResumed testing.encoreTestResumed
func encoreTestResumed(t *testing.T) {
	g := encoreGetG()
	if g == nil || g.req == nil || g.req.data.Test == nil {
		panic("encoreTestResumed: no active test")
	}
	if g.req.data.Test.Current != t {
		panic("encoreTestResumed: active test is not this test")
	}

	// Tests get paused when they call `t.Parallel()` and are held there until the parent test
	// completes, at which case all parallel child tests are resumed.
	// As such, we assume that the test actually "starts" from now
	g.req.data.Start = time.Now()
}

// encoreTestEnd is called when a test ends. This allows Encore's testing framework to clear down any state from the test
// and to perform any assertions on that state that it needs to.
//go:linkname encoreTestEnd testing.encoreTestEnd
func encoreTestEnd(t *testing.T) {
	g := encoreGetG()
	if g == nil || g.req == nil || g.req.data.Test == nil {
		panic("encoreTestEnd: no active test")
	}
	if g.req.data.Test.Current != t {
		panic("encoreTestEnd: active test is not this test")
	}
	testData := g.req.data.Test

	// Wait for any async code to finish up-to 30 seconds
	// if any async code is still running after 30 seconds, we'll fail the test
	done := make(chan struct{})
	go func() {
		testData.Wait.Wait()
		done <- struct{}{}
	}()
	select {
	case <-time.After(30 * time.Second):
		t.Errorf("test timed out waiting for async code to finish after 30 seconds")
		t.Fail()

		// Now cancel to context to try and force those go-routines to exit
		testData.Cancel()
	case <-done:
	}

	encoreCompleteReq()
}

// RunAsyncCodeInTest allows us to trigger code to run asynchronously in a test
// to emulate real world async race scenarios.
//
// This works by running `f` in a new Go routine which can process the "request"
// however, the test will not be able to finish until the go runtime exits
func RunAsyncCodeInTest(t *testing.T, f func(ctx context.Context)) {
	g := encoreGetG()
	if g == nil || g.req == nil || g.req.data.Test == nil {
		panic("encoreTestEnd: no active test")
	}
	if g.req.data.Test.Current != t {
		panic("encoreTestEnd: active test is not this test")
	}
	testData := g.req.data.Test

	testData.Wait.Add(1)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				t.Errorf("panic occured: %v\n\n%s", err, debug.Stack())
				t.Fail()
			}

			testData.Wait.Done()
		}()

		f(testData.Ctx)
	}()
}

// CurrentTest returns the current test.
// If we're not currently inside a test, a panic is raised
func CurrentTest() *testing.T {
	g := encoreGetG()
	if g == nil {
		panic("CurrentTest: no g")
	}
	if g.req == nil {
		panic("CurrentTest: no active operation")
	}
	if g.req.data.Test == nil {
		panic("CurrentTest: no active test")
	}

	return g.req.data.Test.Current
}
