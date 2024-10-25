package test_test

import (
	"testing"

	"encr.dev/internal/httpcache"
	"encr.dev/internal/httpcache/test"
)

func TestMemoryCache(t *testing.T) {
	test.Cache(t, httpcache.NewMemoryCache())
}
