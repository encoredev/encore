//go:build encore_app

package testsupport

import (
	"testing"
	_ "unsafe" // for go:linkname
)

var Singleton *Manager

//go:linkname encoreStartTest testing.encoreStartTest
func encoreStartTest(t *testing.T) {
	Singleton.StartTest(t)
}

//go:linkname encorePauseTest testing.encorePauseTest
func encorePauseTest(t *testing.T) {
	Singleton.PauseTest(t)
}

//go:linkname encoreResumeTest testing.encoreResumeTest
func encoreResumeTest(t *testing.T) {
	Singleton.ResumeTest(t)
}

//go:linkname encoreEndTest testing.encoreEndTest
func encoreEndTest(t *testing.T) {
	Singleton.EndTest(t)
}
