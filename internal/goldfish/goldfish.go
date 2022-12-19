// Package goldfish provides a short-term cache of values.
package goldfish

import (
	"sync"
	"time"
)

type Cache[V any] struct {
	keepalive time.Duration
	fn        func() (V, error)
	mu        sync.Mutex
	last      time.Time
	val       V
}

func New[V any](keepalive time.Duration, fn func() (V, error)) *Cache[V] {
	return &Cache[V]{
		keepalive: keepalive,
		fn:        fn,
	}
}

func (c *Cache[V]) Get() (V, error) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	if now.Sub(c.last) < c.keepalive {
		return c.val, nil
	}

	// Cache is out of date, re-fetch
	val, err := c.fn()
	if err == nil {
		c.val, c.last = val, now
	}
	return c.val, err
}

func (c *Cache[V]) Set(val V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.val = val
	c.last = time.Now()
}
