package limiter

import (
	"context"
)

// noopLimiter is a Limiter that never limits the request
type noopLimiter struct{}

var _ Limiter = noopLimiter{}

func (n noopLimiter) Wait(ctx context.Context) error {
	// We return the context error here, so if the context was cancelled, we'll
	// behave the same as a rate limiter would.
	return ctx.Err()
}
