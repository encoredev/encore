// Package limiter provides a simple interface for rate limiting requests.
package limiter

import (
	"context"

	"golang.org/x/time/rate"

	"encore.dev/appruntime/exported/config"
)

// Limiter is an interface for rate limiting requests.
//
// We're using an interface here instead of [golang.org/x/time/rate.Limiter] so that
// we can easily swap out the implementation in the future for something like doorman.
type Limiter interface {
	// Wait blocks until the limiter is ready to accept a new request, or the context is canceled.
	//
	// If an error is returned, the request being limited should be aborted.
	Wait(ctx context.Context) error
}

// New creates a new [Limiter] based on the given configuration.
//
// A nil configuration will result in a no-op limiter which allows all requests.
func New(cfg *config.Limiter) Limiter {
	if cfg == nil {
		return noopLimiter{}
	}

	switch {
	case cfg.TokenBucket != nil:
		return rate.NewLimiter(rate.Limit(cfg.TokenBucket.PerSecondRate), cfg.TokenBucket.BucketSize)
	default:
		return noopLimiter{}
	}
}
