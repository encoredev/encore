package transport

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"golang.org/x/net/http2"
)

func NewH2CTransport(defaultTransport http.RoundTripper) http.RoundTripper {
	return &H2CTransport{
		h2c: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
		def: defaultTransport,
	}
}

type H2CTransport struct {
	h2c http.RoundTripper
	def http.RoundTripper
}

func (h *H2CTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Scheme == "http" && request.ProtoMajor == 2 {
		return h.h2c.RoundTrip(request)
	}
	return h.def.RoundTrip(request)
}

var _ http.RoundTripper = (*H2CTransport)(nil)
