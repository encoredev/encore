package echo

import (
	"context"
	"os"
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
