// Package dashproxy proxies requests to the dash server,
// caching them locally for offline access.
package dashproxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/peterbourgon/diskv"

	"encr.dev/internal/conf"
	"encr.dev/internal/version"
)

func New(targetURL string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, errors.Wrap(err, "parse target url")
	}

	// Set the cli_version query parameter.
	vals := target.Query()
	vals.Set("cli_version", version.Version)
	target.RawQuery = vals.Encode()

	transport := http.DefaultTransport
	if conf.CacheDevDash {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return nil, errors.Wrap(err, "get user cache dir")
		}

		cache := diskcache.NewWithDiskv(diskv.New(diskv.Options{
			BasePath:     filepath.Join(cacheDir, "encore", "dashcache"),
			CacheSizeMax: 1024 * 1024 * 1024, // 1GiB
			Compression:  diskv.NewGzipCompression(),
		}))
		transport = httpcache.NewTransport(cache)
	}

	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)

			// Configure cache headers so the cache behaves the way we want it to.
			r.Out.Header.Set("Cache-Control", "stale-if-error")
		},
		ModifyResponse: func(resp *http.Response) error {
			if resp.StatusCode < 300 {
				resp.Header.Set("Cache-Control", "max-age=60")
			}
			return nil
		},
	}

	return proxy, nil
}
