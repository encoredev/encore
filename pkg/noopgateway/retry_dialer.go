package noopgateway

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
)

// RetryDialer wraps another dialer and adds exponential backoff
// in the case of connection refused errors.
type RetryDialer struct {
	net.Dialer
	MaxBackoff time.Duration
}

func (d *RetryDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var conn net.Conn
	b := backoff.NewExponentialBackOff()

	b.MaxElapsedTime = d.MaxBackoff
	if b.MaxElapsedTime == 0 {
		b.MaxElapsedTime = time.Minute // Set maximum backoff time to 1 minute
	}

	operation := func() error {
		var err error
		conn, err = d.Dialer.DialContext(ctx, network, address)
		if err != nil {
			if strings.Contains(err.Error(), "connection refused") {
				// Retry if connection is refused
				return err
			}

			// Don't retry if connection isn't refused
			return backoff.Permanent(err)
		}

		return nil
	}

	err := backoff.Retry(operation, b)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
