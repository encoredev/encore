package health

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

// CheckRegistry is a registry of health checks from the API and Infra SDKs
// and other parts of the runtime.
type CheckRegistry struct {
	checks []Check
	m      sync.Mutex
}

// NewCheckRegistry creates a new CheckRegistry.
//
// If running in an app there is a [Singleton]
func NewCheckRegistry() *CheckRegistry {
	return &CheckRegistry{}
}

// Register registers a new health check.
//
// Checks must complete within 5 seconds, otherwise
// they will be terminated and considered failed.
//
// Checks can be called at any time and could have
// multiple goroutines calling them concurrently.
func (c *CheckRegistry) Register(check Check) {
	c.m.Lock()
	defer c.m.Unlock()
	c.checks = append(c.checks, check)
}

// RegisterFunc registers a new health check from a function with a given name
//
// Checks must complete within 5 seconds, otherwise
// they will be terminated and considered failed.
//
// Checks can be called at any time and could have
// multiple goroutines calling them concurrently.
func (c *CheckRegistry) RegisterFunc(name string, check func(ctx context.Context) error) {
	c.m.Lock()
	defer c.m.Unlock()
	c.Register(&checkFunc{name, check})
}

// GetChecks returns all registered health checks.
func (c *CheckRegistry) GetChecks() []Check {
	c.m.Lock()
	defer c.m.Unlock()
	return c.checks
}

// RunAll runs all health checks and returns the results.
func (c *CheckRegistry) RunAll(ctx context.Context) []CheckResult {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	checks := c.GetChecks()

	// Run all checks in parallel.
	results := make(chan []CheckResult, len(checks))
	var wg sync.WaitGroup
	wg.Add(len(checks))
	for _, check := range checks {
		check := check
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Error().Any("panic", r).Msg("health check resulted in a panic")
				}
			}()

			results <- check.HealthCheck(ctx)
		}()
	}

	// Wait for all checks to complete for the context to be cancelled.
	var allResults []CheckResult

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for all checks to complete or the context to be cancelled.
	select {
	case <-done:
	case <-ctx.Done():
		allResults = append(allResults, CheckResult{
			Name: "health-checks.run",
			Err:  ctx.Err(),
		})
	}
	close(results) // then close the results channel

	// Collect results.
	for results := range results {
		allResults = append(allResults, results...)
	}

	// Sort results by name.
	slices.SortFunc(allResults, func(a, b CheckResult) bool {
		return a.Name < b.Name
	})

	return allResults
}
