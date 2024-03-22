package run

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"time"

	"encore.dev/appruntime/exported/config"
)

// ServeHTTP implements http.Handler by forwarding the request to the currently running process.
func (r *Run) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	proc := r.proc.Load().(*ProcGroup)
	proc.ProxyReq(w, req)
}

func addAuthKeyToRequest(req *http.Request, authKey config.EncoreAuthKey) {
	if req.Header == nil {
		req.Header = make(http.Header)
	}

	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)

	mac := hmac.New(sha256.New, authKey.Data)
	_, _ = fmt.Fprintf(mac, "%s\x00%s", date, req.URL.Path)

	bytes := make([]byte, 4, 4+sha256.Size)
	binary.BigEndian.PutUint32(bytes[0:4], authKey.KeyID)
	bytes = mac.Sum(bytes)
	auth := base64.RawStdEncoding.EncodeToString(bytes)
	req.Header.Set("X-Encore-Auth", auth)
}

const TestHeaderDisablePlatformAuth = "X-Encore-Test-Disable-Platform-Auth"
