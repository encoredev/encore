package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"encore.dev/runtime/config"
)

const traceVersion = "5"

func RecordTrace(ctx context.Context, traceID [16]byte, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", config.Cfg.Runtime.TraceEndpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("X-Encore-App-ID", config.Cfg.Runtime.AppID)
	req.Header.Set("X-Encore-Env-ID", config.Cfg.Runtime.EnvID)
	req.Header.Set("X-Encore-Trace-ID", base64.RawStdEncoding.EncodeToString(traceID[:]))
	req.Header.Set("X-Encore-Trace-Version", traceVersion)

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
