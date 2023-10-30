package testsupport

import (
	"context"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	_ "unsafe" // for go:linkname

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/shared/reqtrack"
)

type Manager struct {
	static     *config.Static
	rt         *reqtrack.RequestTracker
	rootLogger zerolog.Logger

	wd              string
	testServiceOnce sync.Once
	testService     string
	testServiceNum  uint16
}

func NewManager(static *config.Static, rt *reqtrack.RequestTracker, rootLogger zerolog.Logger) *Manager {
	wd, _ := os.Getwd()
	return &Manager{static: static, rt: rt, rootLogger: rootLogger, wd: wd}
}

// StartTest is called when a test starts running. This allows Encore's testing framework to
// isolate behavior between different tests on global state.
func (mgr *Manager) StartTest(t *testing.T) {
	var parent *model.Request
	if curr := mgr.rt.Current(); curr.Req != nil {
		parent = curr.Req
	}

	spanID, err := model.GenSpanID()
	if err != nil {
		t.Fatalf("encoreStartTest: failed to generate span ID: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	logger := mgr.rootLogger.With().Str("test", t.Name()).Logger()

	testService, svcNum := mgr.TestService()

	req := &model.Request{
		Type:   model.Test,
		SpanID: spanID,
		Start:  time.Now(),
		Traced: false,
		Test: &model.TestData{
			Ctx:     ctx,
			Cancel:  cancel,
			Current: t,
			Parent:  parent,
			Service: testService,
		},
		Logger: &logger,
		SvcNum: svcNum,
	}
	mgr.rt.BeginRequest(req)
}

// PauseTest is called when a test is paused. This allows Encore's testing framework to
// isolate behavior between different tests on global state.
func (mgr *Manager) PauseTest(t *testing.T) {
}

// ResumeTest is called when a test is resumed after being paused. This allows Encore's testing framework to clear down any state from the test
// and to perform any assertions on that state that it needs to.
func (mgr *Manager) ResumeTest(t *testing.T) {
	req := mgr.rt.Current().Req
	if req == nil || req.Test == nil {
		panic("encoreResumeTest: no active test")
	}
	if req.Test.Current != t {
		panic("encoreResumeTest: active test is not this test")
	}

	// Tests get paused when they call `t.Parallel()` and are held there until the parent test
	// completes, at which case all parallel child tests are resumed.
	// As such, we assume that the test actually "starts" from now
	req.Start = time.Now()
}

// EndTest is called when a test ends. This allows Encore's testing framework to clear down any state from the test
// and to perform any assertions on that state that it needs to.
func (mgr *Manager) EndTest(t *testing.T) {
	req := mgr.rt.Current().Req
	if req == nil || req.Test == nil {
		panic("encoreEndTest: no active test")
	}
	if req.Test.Current != t {
		panic("encoreEndTest: active test is not this test")
	}
	testData := req.Test

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

	mgr.rt.FinishRequest()
}

// CurrentTest returns the currently running test.
// If no test is running, it panics.
func (mgr *Manager) CurrentTest() *testing.T {
	td := mgr.current()
	return td.Current
}

// current returns the currently running test data.
// If no test is running, it panics.
func (mgr *Manager) current() *model.TestData {
	req := mgr.rt.Current().Req
	if req == nil || req.Test == nil {
		panic("CurrentTest: no active test")
	}
	return req.Test
}

func (mgr *Manager) TestService() (svcName string, svcNum uint16) {
	mgr.testServiceOnce.Do(func() {
		for svc, path := range mgr.static.TestServiceMap {
			if mgr.wd == path || strings.HasPrefix(mgr.wd, path+string(filepath.Separator)) {
				mgr.testService = svc
				mgr.testServiceNum = uint16(slices.Index(mgr.static.BundledServices, svc) + 1)
				break
			}
		}
	})

	return mgr.testService, mgr.testServiceNum
}

// RunAsyncCodeInTest allows us to trigger code to run asynchronously in a test
// to emulate real world async race scenarios.
//
// This works by running `f` in a new Go routine which can process the "request"
// however, the test will not be able to finish until the go runtime exits
func (mgr *Manager) RunAsyncCodeInTest(t *testing.T, f func(ctx context.Context)) {
	td := mgr.current()
	if td.Current != t {
		panic("RunAsyncCodeInTest: active test is not this test")
	}
	td.Wait.Add(1)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				t.Errorf("panic occured: %v\n\n%s", err, debug.Stack())
				t.Fail()
			}

			td.Wait.Done()
		}()

		f(td.Ctx)
	}()
}
