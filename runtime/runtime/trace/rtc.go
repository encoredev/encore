package trace

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"time"

	"encore.dev/runtime/config"
)

const traceVersion = "7"

func RecordTrace(ctx context.Context, traceID [16]byte, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", config.Cfg.Runtime.TraceEndpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("X-Encore-App-ID", config.Cfg.Runtime.AppID)
	req.Header.Set("X-Encore-Env-ID", config.Cfg.Runtime.EnvID)
	req.Header.Set("X-Encore-Deploy-ID", config.Cfg.Runtime.DeployID)
	req.Header.Set("X-Encore-App-Commit", config.Cfg.Static.AppCommit.AsRevisionString())
	req.Header.Set("X-Encore-Trace-ID", base64.RawStdEncoding.EncodeToString(traceID[:]))
	req.Header.Set("X-Encore-Trace-Version", traceVersion)
	addAuthKey(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %s: %s", resp.Status, body)
	}
	return nil
}

func addAuthKey(req *http.Request) {
	k := config.Cfg.Runtime.AuthKeys[0]
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)

	mac := hmac.New(sha256.New, k.Data)
	fmt.Fprintf(mac, "%s\x00%s", date, req.URL.Path)

	bytes := make([]byte, 4, 4+sha256.Size)
	binary.BigEndian.PutUint32(bytes[0:4], k.KeyID)
	bytes = mac.Sum(bytes)
	auth := base64.RawStdEncoding.EncodeToString(bytes)
	req.Header.Set("X-Encore-Auth", auth)
}
