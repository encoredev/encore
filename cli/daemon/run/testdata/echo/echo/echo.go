package echo

import (
	"context"
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
