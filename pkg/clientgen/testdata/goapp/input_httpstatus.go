-- go.mod --
module app

-- encore.app --
{"id": ""}

-- svc/svc.go --
package svc

type Response struct {
    Message string
    Status int `encore:"httpstatus"`
}

-- svc/api.go --
package svc

import (
    "context"
    "net/http"
)

// DummyAPI is a dummy endpoint.
//encore:api public
func DummyAPI(ctx context.Context) (*Response, error) {
    return nil, nil
}
