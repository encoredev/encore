package health

import (
	"context"
)

// Check is an interface that can be implemented by any type that wants to be
// registered as a health check.
type Check interface {
	// HealthCheck returns a slice of CheckResult structs, one for each check
	// that was performed.
	HealthCheck(ctx context.Context) []CheckResult
}

// CheckResult is a struct that contains the result of a health check.
type CheckResult struct {
	Name string // Name is the name of the check.
	Err  error  // Err is the error returned by the check (nil for healthy)
}

// checkFunc is a type that implements the Check interface.
type checkFunc struct {
	name  string
	check func(ctx context.Context) error
}

func (c *checkFunc) HealthCheck(ctx context.Context) []CheckResult {
	return []CheckResult{{Name: c.name, Err: c.check(ctx)}}
}
