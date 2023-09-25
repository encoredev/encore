package apiproxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime"

	"github.com/cockroachdb/errors"
	"golang.org/x/oauth2"

	"encr.dev/internal/conf"
	"encr.dev/internal/version"
)

func New(targetURL string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, errors.Wrap(err, "parse target url")
	}

	proxy := &httputil.ReverseProxy{
		Transport: &oauth2.Transport{
			Base:   http.DefaultTransport,
			Source: oauth2.ReuseTokenSource(nil, conf.DefaultTokenSource),
		},
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL = target
			r.Out.Header.Set("User-Agent", "EncoreCLI/"+version.Version)
			r.Out.Header.Set("X-Encore-Dev-Dash", "true")
			r.Out.Header.Set("X-Encore-Version", version.Version)
			r.Out.Header.Set("X-Encore-GOOS", runtime.GOOS)
			r.Out.Header.Set("X-Encore-GOARCH", runtime.GOARCH)
		},
	}
	return proxy, nil
}
