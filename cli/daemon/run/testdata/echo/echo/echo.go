package echo

import "context"

type Data struct {
	Message string
}

// Echo echoes back the request data.
//encore:api public
func Echo(ctx context.Context, params *Data) (*Data, error) {
	return params, nil
}
