package gcsemu

import (
	"context"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

// NewClient returns either a real *storage.Cient, or else a *storage.Client that routes
// to a local emulator if a `GCS_EMULATOR_HOST` environment variable is configured.
func NewClient(ctx context.Context) (*storage.Client, error) {
	if host := os.Getenv("GCS_EMULATOR_HOST"); host != "" {
		return NewTestClientWithHost(ctx, "http://"+host)
	}
	return storage.NewClient(ctx)
}

// NewTestClientWithHost returns a new Google storage client that connects to the given host:port address.
func NewTestClientWithHost(ctx context.Context, hostUrl string) (*storage.Client, error) {
	delegate := http.DefaultTransport
	httpClient := &http.Client{
		Transport: tripperFunc(func(r *http.Request) (*http.Response, error) {
			r = r.Clone(r.Context())
			r.URL.Host = strings.TrimPrefix(hostUrl, "http://")
			r.URL.Scheme = "http"
			return delegate.RoundTrip(r)
		}),
	}
	return storage.NewClient(ctx, option.WithHTTPClient(httpClient))
}

type tripperFunc func(*http.Request) (*http.Response, error)

func (f tripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
