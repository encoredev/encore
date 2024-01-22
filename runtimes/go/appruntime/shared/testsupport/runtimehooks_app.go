//go:build encore_app

package testsupport

import (
	"strings"
	"testing"
	_ "unsafe" // for go:linkname

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/logging"
	"encore.dev/appruntime/shared/reqtrack"
)

var Singleton = NewManager(appconf.Static, reqtrack.Singleton, logging.RootLogger)

func isGeneratedWrapperTest(t *testing.T) bool {
	// A test with an empty name is the generated wrapper test that Go adds around all the users tests.
	// we don't want to treat this as a real test, so we ignore it.
	return t.Name() == ""
}

//go:linkname encoreStartTest testing.encoreStartTest
func encoreStartTest(t *testing.T, fn func(t *testing.T)) {
	if isGeneratedWrapperTest(t) {
		return
	}
	Singleton.StartTest(t, fn)
}

//go:linkname encorePauseTest testing.encorePauseTest
func encorePauseTest(t *testing.T) {
	if isGeneratedWrapperTest(t) {
		return
	}
	Singleton.PauseTest(t)
}

//go:linkname encoreResumeTest testing.encoreResumeTest
func encoreResumeTest(t *testing.T) {
	if isGeneratedWrapperTest(t) {
		return
	}
	Singleton.ResumeTest(t)
}

//go:linkname encoreEndTest testing.encoreEndTest
func encoreEndTest(t *testing.T) {
	if isGeneratedWrapperTest(t) {
		return
	}
	Singleton.EndTest(t)
}

//go:linkname encoreTestLog testing.encoreTestLog
func encoreTestLog(line string, frameSkip int) {
	curr := Singleton.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.LogMessage(trace2.LogMessageParams{
			EventParams: trace2.EventParams{
				TraceID: curr.Req.TraceID,
				SpanID:  curr.Req.SpanID,
				Goid:    curr.Goctr,
			},
			Level: model.LevelTrace,

			// Note all trace logs have 4 spaces added to every line, so we don't want "trimspace"
			Msg:    strings.TrimRight(line, " \n\r\t"),
			Stack:  stack.Build(frameSkip + 1),
			Fields: nil,
		})
	}
}
