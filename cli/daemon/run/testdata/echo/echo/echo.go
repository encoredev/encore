package echo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"encore.dev"
)

type Data struct {
	Message string
}

// Echo echoes back the request data.
//encore:api public
func Echo(ctx context.Context, params *Data) (*Data, error) {
	return params, nil
}

type EnvResponse struct {
	Env []string
}

// Env returns the environment.
//encore:api public
func Env(ctx context.Context) (*EnvResponse, error) {
	return &EnvResponse{Env: os.Environ()}, nil
}

// Raw is a raw endpoint.
//encore:api public raw
func Raw(w http.ResponseWriter, req *http.Request) {
	w2 := httptest.NewRecorder()
	RawEcho(w2, req)
	fmt.Fprintln(w, req.Method, req.URL.Path, w2.Code, w2.Body.String())
}

// RawEcho is a raw endpoint that echoes its body.
//encore:api public raw
func RawEcho(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(205)
	io.Copy(w, req.Body)
}

type AppMetadata struct {
	AppID      string
	APIBaseURL string
	EnvName    string
	EnvType    string
}

// AppMeta returns app metadata.
//encore:api public
func AppMeta(ctx context.Context) (*AppMetadata, error) {
	md := encore.Meta()
	return &AppMetadata{
		AppID:      md.AppID,
		APIBaseURL: md.APIBaseURL.String(),
		EnvName:    md.Environment.Name,
		EnvType:    string(md.Environment.Type),
	}, nil
}
