package platform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"encore.dev/appruntime/exported/trace2"
)

func (c *Client) StreamTrace(log trace2.Logger) error {
	if c.static.Testing {
		// In testing we want to block the test until the trace is done.
		return c.blockingTrace(log)
	} else {
		return c.streamingTrace(log)
	}
}

// streamingTrace streams a trace to the platform.
func (c *Client) streamingTrace(log trace2.Logger) error {
	// Wait a bit for the trace to start, so we can avoid the overhead
	// of small chunk streaming if the trace is short.
	done := log.WaitAtLeast(1 * time.Second)
	var body io.Reader
	if done {
		data, _ := log.GetAndClear()
		if len(data) == 0 {
			return nil
		}

		// Use a bytes.Reader so net/http knows the Content-Length.
		body = bytes.NewReader(data)
	} else {
		r := &traceLogReader{log: log}
		if r.IsDoneAndEmpty() {
			// We didn't get any trace data; don't bother streaming.
			return nil
		}
		body = r
	}

	// Use a background context since the trace is streaming,
	// and we don't know how long it will take to complete.
	ctx := context.Background()
	return c.sendTraceRequest(ctx, body)
}

// blockingTrace waits for the trace to complete before sending it.
func (c *Client) blockingTrace(log trace2.Logger) error {
	// Wait for the trace to complete
	log.WaitUntilDone()
	data, _ := log.GetAndClear()
	if len(data) == 0 {
		return nil // optimization
	}
	body := bytes.NewReader(data)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return c.sendTraceRequest(ctx, body)
}

func (c *Client) sendTraceRequest(ctx context.Context, body io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.runtime.TraceEndpoint, body)
	if err != nil {
		return err
	}

	ta, err := trace2.NewTimeAnchorNow().MarshalText()
	if err != nil {
		return err
	}

	req.Header.Set("X-Encore-App-ID", c.runtime.AppID)
	req.Header.Set("X-Encore-Env-ID", c.runtime.EnvID)
	req.Header.Set("X-Encore-Deploy-ID", c.runtime.DeployID)
	req.Header.Set("X-Encore-App-Commit", c.static.AppCommit.AsRevisionString())
	req.Header.Set("X-Encore-Trace-Version", strconv.Itoa(int(trace2.CurrentVersion)))
	req.Header.Set("X-Encore-Trace-TimeAnchor", string(ta))
	c.addAuthKey(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %s: %s", resp.Status, body)
	}
	return nil
}

// traceLogReader implements io.Reader by reading from a trace log.
type traceLogReader struct {
	log  trace2.Logger
	data []byte // current buffer
	done bool
}

func (r *traceLogReader) Read(b []byte) (int, error) {
	r.readMoreIfNeeded()
	// Post-condition: we have data, or we're done (or both).

	if len(r.data) > 0 {
		n := copy(b, r.data)
		r.data = r.data[n:]
		return n, nil
	} else {
		return 0, io.EOF
	}
}

// IsDoneAndEmpty blocks until we have some trace data, and then
// reports whether the trace is done and empty.
func (r *traceLogReader) IsDoneAndEmpty() bool {
	r.readMoreIfNeeded()
	return len(r.data) == 0 && r.done
}

func (r *traceLogReader) readMoreIfNeeded() {
	for len(r.data) == 0 && !r.done {
		r.data, r.done = r.log.WaitAndClear()
	}
	// Post-condition: we have data, or we're done (or both).
}
