package run

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"encore.dev/appruntime/exported/config"
)

// ServeHTTP implements http.Handler by forwarding the request to the currently running process.
func (r *Run) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	endpoint := strings.TrimLeft(req.URL.Path, "/")
	if endpoint == "" {
		// If this appears to be a browser, serve a redirect to the dashboard.
		dashURL := fmt.Sprintf("http://localhost:%d/%s", r.Mgr.DashPort, r.App.PlatformOrLocalID())
		if ua := req.Header.Get("User-Agent"); strings.Contains(ua, "Gecko") {
			http.Redirect(w, req, dashURL, http.StatusFound)
			return
		}
	}

	proc := r.proc.Load().(*ProcGroup)
	proc.forwardReq(endpoint, w, req)
}

// forwardReq forwards the request to the Encore app.
func (pg *ProcGroup) forwardReq(endpoint string, w http.ResponseWriter, req *http.Request) {
	// director is a simplified version from httputil.NewSingleHostReverseProxy.
	director := func(r *http.Request) {
		r.URL.Scheme = "http"
		r.URL.Host = pg.Run.ListenAddr
		r.URL.Path = "/" + endpoint
		r.URL.RawQuery = req.URL.RawQuery
		if _, ok := r.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			r.Header.Set("User-Agent", "")
		}

		// Add the auth key unless the test header is set.
		if r.Header.Get(TestHeaderDisablePlatformAuth) == "" {
			addAuthKeyToRequest(r, pg.authKey)
		}
	}

	// Create a transport that connects over yamux.
	// Normally transports should be long-lived, but since we disable keep-alives
	// and don't create real TCP connections we can get away with this.
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return pg.Gateway.Client.Open()
		},
	}

	(&httputil.ReverseProxy{
		Director:  director,
		Transport: transport,
	}).ServeHTTP(w, req)
}

func addAuthKeyToRequest(req *http.Request, authKey config.EncoreAuthKey) {
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)

	mac := hmac.New(sha256.New, authKey.Data)
	fmt.Fprintf(mac, "%s\x00%s", date, req.URL.Path)

	bytes := make([]byte, 4, 4+sha256.Size)
	binary.BigEndian.PutUint32(bytes[0:4], authKey.KeyID)
	bytes = mac.Sum(bytes)
	auth := base64.RawStdEncoding.EncodeToString(bytes)
	req.Header.Set("X-Encore-Auth", auth)
}

const TestHeaderDisablePlatformAuth = "X-Encore-Test-Disable-Platform-Auth"
