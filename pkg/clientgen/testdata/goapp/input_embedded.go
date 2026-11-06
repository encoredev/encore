-- go.mod --
module app

-- encore.app --
{"id": ""}

-- svc/svc.go --
package svc

import "context"

//encore:service
type Service struct{}

type ExampleEmbedded struct {
	ExampleFieldInner string
}

type ExampleOuter struct {
	ExampleEmbedded
	ExampleFieldOuter string
}

type ExampleResponse struct {
	Example ExampleOuter
}

//encore:api public
func (s *Service) Example(ctx context.Context) (*ExampleResponse, error) {
	return &ExampleResponse{
		Example: ExampleOuter{
			ExampleEmbedded: ExampleEmbedded{
				ExampleFieldInner: "inner",
			},
			ExampleFieldOuter: "outer",
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Extensions: the same types as above, used in the other places an API type
// can appear. Nothing new is introduced except where a bug needs it.
// ---------------------------------------------------------------------------

// ExampleEcho receives an ExampleOuter as a POST body and returns it
// inside ExampleResponse, so the reply shows what the handler received.
//
//encore:api public method=POST
func (s *Service) ExampleEcho(ctx context.Context, p *ExampleOuter) (*ExampleResponse, error) {
	return &ExampleResponse{Example: *p}, nil
}

// ExampleQuery receives an ExampleOuter as GET query parameters and
// returns it the same way.
//
//encore:api public method=GET path=/example/query
func (s *Service) ExampleQuery(ctx context.Context, p *ExampleOuter) (*ExampleResponse, error) {
	return &ExampleResponse{Example: *p}, nil
}

// ExampleTop returns ExampleOuter directly, as the top-level response.
//
//encore:api public
func (s *Service) ExampleTop(ctx context.Context) (*ExampleOuter, error) {
	return &ExampleOuter{
		ExampleEmbedded: ExampleEmbedded{
			ExampleFieldInner: "inner",
		},
		ExampleFieldOuter: "outer",
	}, nil
}

// Two other ways to embed ExampleEmbedded: through a pointer, and with a JSON tag.

type ExamplePtrOuter struct {
	*ExampleEmbedded
	ExampleFieldOuter string
}

type ExampleTaggedOuter struct {
	ExampleEmbedded   `json:"example_embedded"`
	ExampleFieldOuter string
}

type ExampleVariantsResponse struct {
	Ptr    ExamplePtrOuter
	Tagged ExampleTaggedOuter
}

//encore:api public
func (s *Service) ExampleVariants(ctx context.Context) (*ExampleVariantsResponse, error) {
	return &ExampleVariantsResponse{
		Ptr: ExamplePtrOuter{
			ExampleEmbedded:   &ExampleEmbedded{ExampleFieldInner: "inner"},
			ExampleFieldOuter: "outer",
		},
		Tagged: ExampleTaggedOuter{
			ExampleEmbedded:   ExampleEmbedded{ExampleFieldInner: "inner"},
			ExampleFieldOuter: "outer",
		},
	}, nil
}
