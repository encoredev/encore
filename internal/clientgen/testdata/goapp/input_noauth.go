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
