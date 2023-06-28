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
	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog/log"

	"encr.dev/cli/internal/platform/gql"
	"encr.dev/internal/conf"
	"encr.dev/internal/version"
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
	log.Trace().Interface("request", reqParams).Msgf("->     %s %s", method, path)
	defer func() {
		if err != nil {
			log.Trace().Err(err).Msgf("<- ERR %s %s", method, path)
		} else {
			log.Trace().Interface("response", respParams).Msgf("<- OK  %s %s", method, path)
		}
	}()

	resp, err := sendPlatformReq(ctx, method, path, reqParams, auth)
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

type graphqlRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
	Extensions    map[string]interface{} `json:"extensions,omitempty"`
}

var graphqlDecoder = (func() jsoniter.API {
	enc := jsoniter.Config{}.Froze()
	enc.RegisterExtension(NewInterfaceCodecExtension())
	return enc
})()

// graphqlCall makes a GraphQL request.
func graphqlCall(ctx context.Context, req graphqlRequest, respData any, auth bool) (err error) {
	log.Trace().Msgf("->     graphql %s: %+v", req.OperationName, req.Variables)
	httpResp, err := sendPlatformReq(ctx, "POST", "/graphql", req, auth)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	var respStruct struct {
		Data       json.RawMessage
		Errors     gql.ErrorList
		Extensions map[string]interface{}
	}
	defer func() {
		if err != nil {
			log.Trace().Msgf("<- ERR graphql %s: %v", req.OperationName, err)
		} else {
			log.Trace().Msgf("<- OK  graphql %s: %s", req.OperationName, respStruct.Data)
		}
	}()

	if err := json.NewDecoder(httpResp.Body).Decode(&respStruct); err != nil {
		return fmt.Errorf("decode response: %v", err)
	} else if len(respStruct.Errors) > 0 {
		return fmt.Errorf("graphql request failed: %w", respStruct.Errors)
	}
	if respData != nil {
		if err := graphqlDecoder.NewDecoder(bytes.NewReader(respStruct.Data)).Decode(respData); err != nil {
			return fmt.Errorf("decode graphql data: %v", err)
		}
	}
	return nil
}

// rawCall makes a call to the API endpoint given by method and path.
// It returns the raw HTTP response body on success; it must be closed by the caller.
func rawCall(ctx context.Context, method, path string, reqParams interface{}, auth bool) (respBody io.ReadCloser, err error) {
	log.Trace().Msgf("->     %s %s: %+v", method, path, reqParams)
	defer func() {
		if err != nil {
			log.Trace().Msgf("<- ERR %s %s: %v", method, path, err)
		} else {
			log.Trace().Msgf("<- OK  %s %s", method, path)
		}
	}()

	resp, err := sendPlatformReq(ctx, method, path, reqParams, auth)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			resp.Body.Close()
		}
	}()

	if resp.StatusCode >= 400 {
		return nil, decodeErrorResponse(resp)
	}

	return resp.Body, nil
}

func sendPlatformReq(ctx context.Context, method, path string, reqParams any, auth bool) (httpResp *http.Response, err error) {
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

	req, err := http.NewRequestWithContext(ctx, method, conf.APIBaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if reqParams != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return doPlatformReq(req, auth)
}

func doPlatformReq(req *http.Request, auth bool) (httpResp *http.Response, err error) {
	// Add a very limited amount of information for diagnostics
	req.Header.Set("User-Agent", "EncoreCLI/"+version.Version)
	req.Header.Set("X-Encore-Version", version.Version)
	req.Header.Set("X-Encore-GOOS", runtime.GOOS)
	req.Header.Set("X-Encore-GOARCH", runtime.GOARCH)

	client := http.DefaultClient
	if auth {
		client = conf.AuthClient
	}
	return client.Do(req)
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

	log.Trace().Msgf("->     %s %s: %+v", "WS", path, extraHeaders)
	defer func() {
		if err != nil {
			log.Trace().Msgf("<- ERR %s %s: %v", "WS", path, err)
		} else {
			log.Trace().Msgf("<- OK  %s %s", "WS", path)
		}
	}()

	if auth {
		tok, err := conf.DefaultTokenSource.Token()
		if err != nil {
			return nil, err
		}
		header.Set("Authorization", "Bearer "+tok.AccessToken)
	}

	url := conf.WSBaseURL + path
	log.Trace().Msgf("->     %s %s: connecting to %s", "WS", path, url)
	ws, httpResp, err := websocket.DefaultDialer.DialContext(ctx, url, header)
	if httpResp != nil && httpResp.StatusCode >= 400 {
		var respStruct struct {
			OK    bool
			Error Error
			Data  json.RawMessage
		}
		if err := json.NewDecoder(httpResp.Body).Decode(&respStruct); err != nil {
			return nil, fmt.Errorf("decode response: %v", err)
		} else if !respStruct.OK {
			e := respStruct.Error
			e.HTTPCode = httpResp.StatusCode
			e.HTTPStatus = httpResp.Status
			return nil, e
		}
	}

	return ws, err
}

func decodeErrorResponse(resp *http.Response) error {
	var respStruct struct {
		OK    bool
		Error Error
		Data  json.RawMessage
	}
	if err := json.NewDecoder(resp.Body).Decode(&respStruct); err != nil {
		return fmt.Errorf("decode response: %v", err)
	}
	e := respStruct.Error
	e.HTTPCode = resp.StatusCode
	e.HTTPStatus = resp.Status
	return e
}
