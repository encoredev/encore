package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"

	"encr.dev/cli/internal/conf"
	"encr.dev/cli/internal/version"
)

type Error struct {
	HTTPStatus string `json:"-"`
	HTTPCode   int    `json:"-"`
	Code       string
	Detail     json.RawMessage
}

func (e Error) Error() string {
	if len(e.Detail) > 0 {
		return fmt.Sprintf("http %s: code=%s detail=%s", e.HTTPStatus, e.Code, e.Detail)
	}
	return fmt.Sprintf("http %s: code=%s", e.HTTPStatus, e.Code)
}

// call makes a call to the API endpoint given by method and path.
// If reqParams and respParams are non-nil they are JSON-marshalled/unmarshalled.
func call(ctx context.Context, method, path string, reqParams, respParams interface{}) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%s %s: %w", method, path, err)
		}
	}()

	var body io.Reader
	if reqParams != nil {
		reqData, err := json.Marshal(reqParams)
		if err != nil {
			return fmt.Errorf("marshal request: %v", err)
		}
		body = bytes.NewReader(reqData)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", conf.APIBaseURL+path, body)
	if err != nil {
		return err
	}
	if reqParams != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Add a very limited amount of information for diagnostics
	req.Header.Set("User-Agent", "EncoreCLI/"+version.Version)
	req.Header.Set("X-Encore-Version", version.Version)
	req.Header.Set("X-Encore-GOOS", runtime.GOOS)
	req.Header.Set("X-Encore-GOARCH", runtime.GOARCH)

	resp, err := conf.AuthClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var respStruct struct {
		OK    bool
		Error Error
		Data  json.RawMessage
	}
	if err := json.NewDecoder(resp.Body).Decode(&respStruct); err != nil {
		return fmt.Errorf("decode response: %v", err)
	} else if !respStruct.OK {
		e := respStruct.Error
		e.HTTPCode = resp.StatusCode
		e.HTTPStatus = resp.Status
		return e
	}

	if respParams != nil {
		if err := json.Unmarshal([]byte(respStruct.Data), respParams); err != nil {
			return fmt.Errorf("decode response data: %v", err)
		}
	}
	return nil
}

// rawCall makes a call to the API endpoint given by method and path.
// It returns the raw HTTP response body on success; it must be closed by the caller.
func rawCall(ctx context.Context, method, path string, reqParams interface{}) (respBody io.ReadCloser, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%s %s: %w", method, path, err)
		}
	}()

	var body io.Reader
	if reqParams != nil {
		reqData, err := json.Marshal(reqParams)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %v", err)
		}
		body = bytes.NewReader(reqData)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", conf.APIBaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if reqParams != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Add a very limited amount of information for diagnostics
	req.Header.Set("User-Agent", "EncoreCLI/"+version.Version)
	req.Header.Set("X-Encore-Version", version.Version)
	req.Header.Set("X-Encore-GOOS", runtime.GOOS)
	req.Header.Set("X-Encore-GOARCH", runtime.GOARCH)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			resp.Body.Close()
		}
	}()

	if resp.StatusCode >= 400 {
		var respStruct struct {
			OK    bool
			Error Error
			Data  json.RawMessage
		}
		if err := json.NewDecoder(resp.Body).Decode(&respStruct); err != nil {
			return nil, fmt.Errorf("decode response: %v", err)
		}
		e := respStruct.Error
		e.HTTPCode = resp.StatusCode
		e.HTTPStatus = resp.Status
		return nil, e
	}

	return resp.Body, nil
}
