-- go.mod --
module app

-- encore.app --
{"id": ""}

-- svc/svc.go --
package svc

type Request struct {
	Message string
}

-- svc/api.go --
package svc

import (
	"context"
	"net/http"
)

// DummyAPI is a dummy endpoint.
//encore:api public
func DummyAPI(ctx context.Context, req *Request) error {
	return nil
}

// Private is a basic auth endpoint.
//encore:api auth
func Private(ctx context.Context, req *Request) error {
	return nil
}

-- authentication/auth.go --
package authentication

import (
	"context"

	"encore.dev/beta/auth"
)

type User struct {
	ID   int     `json:"id"`
	Name string  `json:"name"`
}

//encore:authhandler
func AuthenticateRequest(ctx context.Context, token string) (auth.UID, *User, error) {
	return "", nil, nil
}
