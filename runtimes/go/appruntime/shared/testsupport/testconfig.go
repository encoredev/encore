package testsupport

import (
	"sync/atomic"

	"encore.dev/appruntime/exported/model"
)

type TestConfig = model.TestConfig

var (
	nextApiMockID atomic.Uint64
)

func newTestConfig(parent *model.TestConfig) *model.TestConfig {
	return &model.TestConfig{
		Parent:       parent,
		ServiceMocks: make(map[string]any),
		APIMocks:     make(map[string]map[string]model.ApiMock),
	}
}

// walkConfig walks the test config hierarchy, starting from the given config, and calls the given function on each
// until the function returns true.
//
// walkConfig takes care to lock and unlock the read mutex on the config hierarchy as it walks it.
func walkConfig[T any](config *model.TestConfig, f func(*model.TestConfig) (value T, found bool)) (value T, found bool) {
	exec := func(config *model.TestConfig) (value T, found bool) {
		config.Mu.RLock()
		defer config.Mu.RUnlock()
		return f(config)
	}

	for config != nil {
		value, found := exec(config)
		if found {
			return value, found
		}

		config = config.Parent
	}

	return value, false
}
