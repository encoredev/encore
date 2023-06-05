package config

import (
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	jsoniter "github.com/json-iterator/go"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/shared/encoreenv"
	"encore.dev/appruntime/shared/reqtrack"
)

type ValueID uint64
type ValuePath []string

type Manager struct {
	// Runtime components we need for config
	rt   *reqtrack.RequestTracker
	json jsoniter.API

	// config tracking systems
	nextValueID atomic.Uint64
	extraction  struct {
		mutex         sync.Mutex   // Used only by GetMetaForValue
		running       atomic.Bool  // Only store when under mutex
		scopeMutex    sync.RWMutex // Mutex for the forSpan / forGoRoutine
		forSpan       model.SpanID // Which span are we extracting for?
		forGoRoutine  uint32       // Which go routine within the span are we extracting for
		count         int          // How many extractions have we done?
		ExtractedID   ValueID      // What's the ValueID we extracted?
		ExtractedPath ValuePath    // What's the path we extracted?
	}

	// Test support
	testMutex     sync.RWMutex
	testOverrides map[*testing.T]map[ValueID]any
}

func NewManager(rt *reqtrack.RequestTracker, json jsoniter.API) *Manager {
	return &Manager{
		rt:            rt,
		json:          json,
		testOverrides: make(map[*testing.T]map[ValueID]any),
	}
}

func (m *Manager) getComputedCUE(serviceName string) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("config subsystem has not been initialized")
	}

	// Fetch the raw JSON config for this service
	envVar := encoreenv.Get(envName(serviceName))
	if envVar == "" {
		return nil, fmt.Errorf("configuration for service `%s` not found, expected it in environmental variable %s", serviceName, envName(serviceName))
	}
	cfgBytes, err := base64.RawURLEncoding.DecodeString(envVar)
	if err != nil {
		return nil, fmt.Errorf("failed to decode configuration for service `%s`: %v", serviceName, err)
	}
	return cfgBytes, nil
}

// nextID returns the next unique ID for a config value to use to be tracked
func (m *Manager) nextID() ValueID {
	if m == nil {
		panic("config subsystem has not been initialized")
	}
	return ValueID(m.nextValueID.Add(1))
}

// valueMeta is used by Values to provide their ID and path to GetMetaForValue
//
// It is called by the Value function itself, and so is called in the context of the
// of a goroutine that is running the GetMetaForValue function. If we are not in
// that goroutine this method has no effect and the value is returned as normal.
func (m *Manager) valueMeta(id ValueID, path ValuePath) {
	// Fast pass if we're not extracting
	if !m.extraction.running.Load() {
		return
	}

	// Check if we're the right Goroutine that we want to extract from
	req := m.rt.Current()
	m.extraction.scopeMutex.RLock()
	defer m.extraction.scopeMutex.RUnlock()
	if req.Req.SpanID != m.extraction.forSpan || req.Goctr != m.extraction.forGoRoutine {
		return
	}

	// We're the right goroutine, so we can store the value
	m.extraction.ExtractedID = id
	m.extraction.ExtractedPath = path
	m.extraction.count++
}

// envName takes a service name and converts it to an environment variable name in which
// the service's configuration JSON is stored at runtime
func envName(serviceName string) string {
	// normalise the name
	serviceName = strings.ToUpper(serviceName)

	return fmt.Sprintf("ENCORE_CFG_%s", serviceName)
}
