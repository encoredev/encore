package svcproxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"

	"encr.dev/pkg/logging"
)

type SvcProxy struct {
	listener   net.Listener
	logger     zerolog.Logger
	httpServer *http.Server

	mu       sync.RWMutex
	gateways map[string]*httputil.ReverseProxy // Map of the gateway name to address and port it's listening on
	services map[string]*httputil.ReverseProxy // Map of service name to address and port it's listening on
}

var (
	_ http.Handler = (*SvcProxy)(nil)
)

// New creates a new service proxy and it starts listening on a random port
//
// You must call Close() on the returned service proxy when you are done with it.
func New(ctx context.Context, logger zerolog.Logger) (*SvcProxy, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, errors.Wrap(err, "unable to listen")
	}

	proxy := &SvcProxy{
		listener: ln,
		logger:   logger,
		gateways: make(map[string]*httputil.ReverseProxy),
		services: make(map[string]*httputil.ReverseProxy),
	}

	proxy.httpServer = &http.Server{
		Addr:        ln.Addr().String(),
		BaseContext: func(_ net.Listener) context.Context { return ctx },
		Handler:     proxy,
	}

	go func() {
		if err := proxy.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Err(err).Msg("error serving")
		}
	}()

	return proxy, nil
}

// Close stops running the service proxy
func (p *SvcProxy) Close() {
	_ = p.httpServer.Close()
	_ = p.listener.Close()
}

func (p *SvcProxy) RegisterGateway(name string, addr netip.AddrPort) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.gateways[name] = p.createReverseProxy("gateway", name, addr)

	return fmt.Sprintf("http://%s/gateway/%s", p.listener.Addr().String(), name)
}

// RegisterService registers a service with the proxy and returns the BaseURL to be used
// to access the service.
func (p *SvcProxy) RegisterService(name string, addr netip.AddrPort) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.services[name] = p.createReverseProxy("service", name, addr)

	return fmt.Sprintf("http://%s/service/%s", p.listener.Addr().String(), name)
}

func (p *SvcProxy) createReverseProxy(what, name string, listener netip.AddrPort) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		// This transport is copied from the default transport in the http package just with the dial context
		// wrapped in our retry dialer.
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&retryDialer{net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Rewrite: func(request *httputil.ProxyRequest) {
			request.Out.URL.Scheme = "http"
			request.Out.URL.Host = listener.String()
			request.Out.URL.Path = strings.TrimPrefix(request.In.URL.Path, fmt.Sprintf("/%s/%s", what, name))
		},
		ErrorLog: logging.NewZeroLogAdapter(p.logger.With().Str(what, name).Logger(), zerolog.ErrorLevel),
	}
}

// ServeHTTP implements the http.Handler interface
func (p *SvcProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Resolve the handler with the lock held.
	proxy, err := func() (http.Handler, error) {
		p.mu.RLock()
		defer p.mu.RUnlock()

		parts := strings.SplitN(strings.TrimPrefix(req.URL.Path, "/"), "/", 3)
		if len(parts) >= 2 {
			switch parts[0] {
			case "gateway":
				proxy, ok := p.gateways[parts[1]]
				if !ok {
					return nil, errors.Newf("unknown gateway: %s", parts[1])
				} else {
					return proxy, nil
				}
			case "service":
				proxy, ok := p.services[parts[1]]
				if !ok {
					return nil, errors.Newf("unknown service: %s", parts[1])
				} else {
					return proxy, nil
				}
			default:
				return nil, errors.Newf("unknown path prefix: %s", parts[0])
			}
		} else {
			return nil, errors.Newf("unknown path prefix format: %s", req.URL.Path)
		}
	}()

	if err != nil {
		p.logger.Err(err).Str("url", req.URL.String()).Msg("error proxying service request")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	proxy.ServeHTTP(w, req)
}
