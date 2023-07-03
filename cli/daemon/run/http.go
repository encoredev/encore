package run

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
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
	proc.Gateway.ProxyReq(w, req)
}

func addAuthKeyToRequest(req *http.Request, authKey config.EncoreAuthKey) {
	if req.Header == nil {
		req.Header = make(http.Header)
	}

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
