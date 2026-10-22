-- go.mod --
module app

require (
	encore.dev v1.52.1
)

-- encore.app --
{"id": ""}

-- svc/svc.go --
// Svc demonstrates named types in query parameters.
package svc

import (
    "context"
)

// Status is a named type based on string.
type Status string

const (
    Active   Status = "active"
    Inactive Status = "inactive"
)

// Priority is another named type based on string.
type Priority string

// QueryNamedTypesRequest uses custom named types as query parameters.
type QueryNamedTypesRequest struct {
    Name  Status   `query:"name"`
    Prio  Priority `query:"prio" encore:"optional"`
    Limit int      `query:"limit" encore:"optional"`
}

// QueryNamedTypesResponse is the response for the named types query endpoint.
type QueryNamedTypesResponse struct {
    Greeting string `json:"greeting"`
}

//encore:api public method=GET path=/named
func QueryNamedTypes(ctx context.Context, req *QueryNamedTypesRequest) (*QueryNamedTypesResponse, error) {
    return &QueryNamedTypesResponse{Greeting: string(req.Name)}, nil
}

//encore:api public method=DELETE
func DeleteNamed(ctx context.Context, req *QueryNamedTypesRequest) (*QueryNamedTypesResponse, error) {
    return &QueryNamedTypesResponse{Greeting: string(req.Name)}, nil
}

// HeaderNamedRequest uses a named type in a header parameter.
type HeaderNamedRequest struct {
    Name Status `header:"X-Name"`
}

//encore:api public method=POST
func HeaderNamed(ctx context.Context, req *HeaderNamedRequest) (*QueryNamedTypesResponse, error) {
    return &QueryNamedTypesResponse{Greeting: string(req.Name)}, nil
}

-- client/client.go --
package client

import (
    "context"
    "fmt"

    "app/svc"
)

//encore:api public method=GET path=/client
func CallNamed(ctx context.Context) (*CallResponse, error) {
    resp, err := svc.QueryNamedTypes(ctx, &svc.QueryNamedTypesRequest{
        Name:  svc.Active,
        Prio:  "high",
        Limit: 5,
    })
    if err != nil {
        return nil, err
    }

    // Also test DELETE with named query types
    delResp, err := svc.DeleteNamed(ctx, &svc.QueryNamedTypesRequest{
        Name:  svc.Inactive,
        Prio:  "low",
    })
    if err != nil {
        return nil, err
    }

    // Also test header with named types
    hdrResp, err := svc.HeaderNamed(ctx, &svc.HeaderNamedRequest{
        Name: svc.Active,
    })
    if err != nil {
        return nil, err
    }

    return &CallResponse{
        Result: fmt.Sprintf("query=%q delete=%q header=%q", resp.Greeting, delResp.Greeting, hdrResp.Greeting),
    }, nil
}

type CallResponse struct {
    Result string `json:"result"`
}
