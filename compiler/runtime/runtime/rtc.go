package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

var (
	runtimeAddr string
	procID      string
)

func RecordTrace(ctx context.Context, traceID [16]byte, data []byte) error {
	req, err := http.NewRequest("POST", runtimeAddr+"/trace", bytes.NewReader(data))
	if err != nil {
		return err
	}
	id := base64.RawURLEncoding.EncodeToString(traceID[:])
	req.Header.Set("Content-Type", "application/vnd.google.protobuf")
	req.Header.Set("X-Encore-Trace-Version", "v3")
	req.Header.Set("X-Encore-Trace-ID", id)
	req.Header.Set("X-Encore-Proc-ID", procID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("could not record trace: HTTP %s: %s", resp.Status, body)
	}
	return nil
}

func init() {
	envs := []string{
		"ENCORE_RUNTIME_ADDRESS",
		"ENCORE_PROC_ID",
	}
	var vals []string
	for _, env := range envs {
		val := os.Getenv(env)
		if val == "" {
			log.Fatalf("encore: internal error: %s not set", env)
		}
		vals = append(vals, val)
	}

	runtimeAddr = "http://" + vals[0]
	procID = vals[1]
}
