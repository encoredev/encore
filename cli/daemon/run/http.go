package run

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
)

// ServeHTTP implements http.Handler by forwarding the request to the currently running process.
func (r *Run) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	endpoint := strings.TrimLeft(req.URL.Path, "/")
	if endpoint == "" {
		// If this appears to be a browser, serve a redirect to the dashboard.
		// Otherwise, give a helpful error message for terminals and such.
		dashURL := fmt.Sprintf("http://localhost:%d/%s", r.mgr.DashPort, r.AppID)
		if ua := req.Header.Get("User-Agent"); strings.Contains(ua, "Gecko") {
			http.Redirect(w, req, dashURL, http.StatusFound)
			return
		}

		http.Error(w, "No endpoint given. Make API calls to /service.Endpoint instead."+
			"Visit the browser dashboard at: "+dashURL, http.StatusBadRequest)
		return
	}

	proc := r.proc.Load().(*Proc)
	proc.forwardReq(endpoint, w, req)
}

// forwardReq forwards the request to the Encore app.
func (p *Proc) forwardReq(endpoint string, w http.ResponseWriter, req *http.Request) {
	if req.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(200)
		return
	}
	// director is a simplified version from httputil.NewSingleHostReverseProxy.
	director := func(r *http.Request) {
		r.URL.Scheme = "http"
		r.URL.Host = "localhost:" + strconv.Itoa(p.Run.Port)
		r.URL.Path = "/" + endpoint
		r.URL.RawQuery = req.URL.RawQuery
		if _, ok := r.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			r.Header.Set("User-Agent", "")
		}
	}
	// modifyResponse sets the appropriate CORS headers for local development.
	modifyResponse := func(r *http.Response) error {
		r.Header.Set("Access-Control-Allow-Origin", "*")
		r.Header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		r.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		return nil
	}

	// Create a transport that connects over yamux.
	// Normally transports should be long-lived, but since we disable keep-alives
	// and don't create real TCP connections we can get away with this.
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return p.client.Open()
		},
	}

	(&httputil.ReverseProxy{
		Director:       director,
		ModifyResponse: modifyResponse,
		Transport:      transport,
	}).ServeHTTP(w, req)
}
