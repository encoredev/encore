package openapi

import (
	"bytes"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/getkin/kin-openapi/openapi3"

	"encr.dev/pkg/clientgen/clientgentypes"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

func TestOpenAPITagsGeneration(t *testing.T) {
	c := qt.New(t)

	// Create test metadata with OpenAPI tags
	md := &meta.Data{
		Svcs: []*meta.Service{
			{
				Name: "users",
				Rpcs: []*meta.RPC{
					{
						Name:        "GetUser",
						ServiceName: "users",
						AccessType:  meta.RPC_PUBLIC,
						Path: &meta.Path{
							Segments: []*meta.PathSegment{
								{Type: meta.PathSegment_LITERAL, Value: "users"},
								{Type: meta.PathSegment_PARAM, Value: "id", ValueType: meta.PathSegment_STRING},
							},
						},
						HttpMethods: []string{"GET"},
						OpenapiTags: []string{"Users", "Management"},
					},
					{
						Name:        "CreateUser",
						ServiceName: "users",
						AccessType:  meta.RPC_PUBLIC,
						Path: &meta.Path{
							Segments: []*meta.PathSegment{
								{Type: meta.PathSegment_LITERAL, Value: "users"},
							},
						},
						HttpMethods: []string{"POST"},
						OpenapiTags: []string{"Users"},
					},
				},
			},
		},
	}

	// Generate OpenAPI spec
	gen := New(Initial)
	params := clientgentypes.GenerateParams{
		Meta:     md,
		AppSlug:  "test-app",
		Services: clientgentypes.NewServiceSet(md, []string{"users"}, []string{}),
		Tags:     clientgentypes.NewTagSet([]string{}, []string{}),
		Options:  clientgentypes.Options{},
		Buf:      &bytes.Buffer{},
	}

	err := gen.Generate(params)
	c.Assert(err, qt.IsNil)

	// Parse generated JSON
	var spec openapi3.T
	err = json.Unmarshal(params.Buf.Bytes(), &spec)
	c.Assert(err, qt.IsNil)

	// Test GetUser endpoint has multiple tags
	getUserOp := spec.Paths["/users/{id}"].Get
	c.Assert(getUserOp, qt.Not(qt.IsNil))
	c.Assert(getUserOp.Tags, qt.DeepEquals, []string{"Users", "Management"})

	// Test CreateUser endpoint has single tag
	createUserOp := spec.Paths["/users"].Post
	c.Assert(createUserOp, qt.Not(qt.IsNil))
	c.Assert(createUserOp.Tags, qt.DeepEquals, []string{"Users"})
}

func TestOpenAPINoTagsGeneration(t *testing.T) {
	c := qt.New(t)

	// Create test metadata without OpenAPI tags
	md := &meta.Data{
		Svcs: []*meta.Service{
			{
				Name: "users",
				Rpcs: []*meta.RPC{
					{
						Name:        "GetUser",
						ServiceName: "users",
						AccessType:  meta.RPC_PUBLIC,
						Path: &meta.Path{
							Segments: []*meta.PathSegment{
								{Type: meta.PathSegment_LITERAL, Value: "users"},
							},
						},
						HttpMethods: []string{"GET"},
						OpenapiTags: []string{}, // No OpenAPI tags
					},
				},
			},
		},
	}

	// Generate OpenAPI spec
	gen := New(Initial)
	params := clientgentypes.GenerateParams{
		Meta:     md,
		AppSlug:  "test-app",
		Services: clientgentypes.NewServiceSet(md, []string{"users"}, []string{}),
		Tags:     clientgentypes.NewTagSet([]string{}, []string{}),
		Options:  clientgentypes.Options{},
		Buf:      &bytes.Buffer{},
	}

	err := gen.Generate(params)
	c.Assert(err, qt.IsNil)

	// Parse generated JSON
	var spec openapi3.T
	err = json.Unmarshal(params.Buf.Bytes(), &spec)
	c.Assert(err, qt.IsNil)

	// Test endpoint has no tags
	getUserOp := spec.Paths["/users"].Get
	c.Assert(getUserOp, qt.Not(qt.IsNil))
	c.Assert(getUserOp.Tags, qt.HasLen, 0)
}
