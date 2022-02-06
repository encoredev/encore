package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

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
func call(ctx context.Context, method, path string, reqParams, respParams interface{}, auth bool) (err error) {
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

	req, err := http.NewRequestWithContext(ctx, method, conf.APIBaseURL+path, body)
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

	client := http.DefaultClient
	if auth {
		client = conf.AuthClient
	}

	log.Trace().Msgf("->     %s %s: %+v", method, path, reqParams)
	defer func() {
		if err != nil {
			log.Trace().Msgf("<- ERR %s %s: %v", method, path, err)
		} else {
			log.Trace().Msgf("<- OK  %s %s: %+v", method, path, respParams)
		}
	}()

	resp, err := client.Do(req)
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
func rawCall(ctx context.Context, method, path string, reqParams interface{}, auth bool) (respBody io.ReadCloser, err error) {
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

	client := http.DefaultClient
	if auth {
		client = conf.AuthClient
	}
	resp, err := client.Do(req)
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

// wsDial sets up a WebSocket conncetion to the API endpoint given by method and path.
func wsDial(ctx context.Context, path string, auth bool, extraHeaders map[string]string) (ws *websocket.Conn, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("WS %s: %w", path, err)
		}
	}()

	// Add a very limited amount of information for diagnostics
	header := make(http.Header)
	header.Set("User-Agent", "EncoreCLI/"+version.Version)
	header.Set("X-Encore-Version", version.Version)
	header.Set("X-Encore-GOOS", runtime.GOOS)
	header.Set("X-Encore-GOARCH", runtime.GOARCH)
	header.Set("Origin", "http://encore-cli.local")
	for k, v := range extraHeaders {
		header.Set(k, v)
	}
	log.Info().Msgf("sending startup data %#v", extraHeaders)

	if auth {
		tok, err := conf.DefaultTokenSource.Token()
		if err != nil {
			return nil, err
		}
		header.Set("Authorization", "Bearer "+tok.AccessToken)
	}

	url := conf.WSBaseURL + path
	ws, _, err = websocket.DefaultDialer.DialContext(ctx, url, header)
	return ws, err
}
