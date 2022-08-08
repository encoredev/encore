//go:build encore_app

package testsupport

import (
	"testing"
	_ "unsafe" // for go:linkname

	_ "encore.dev/appruntime/app/appinit" // Force the app to initialise all singletons before these functions can be used
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
