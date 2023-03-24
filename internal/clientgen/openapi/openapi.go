package openapi

import (
	"bytes"
	"fmt"
	"go/doc/comment"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/getkin/kin-openapi/openapi3"

	"encr.dev/parser/encoding"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

type GenVersion int

const (
	// Initial is the originally released OpenAPI client generator
	Initial GenVersion = iota

	// Experimental can be used to lock experimental or uncompleted features in the generated code
	// It should always be the last item in the enum.
	Experimental

	LatestVersion GenVersion = Experimental - 1
)

type Generator struct {
	ver       GenVersion
	spec      *openapi3.T
	md        *meta.Data
	seenDecls map[string]uint32
}

func New(version GenVersion) *Generator {
	return &Generator{
		ver:       version,
		seenDecls: make(map[string]uint32),
	}
}

func (g *Generator) Version() int {
	return int(g.ver)
}

func (g *Generator) Generate(buf *bytes.Buffer, appSlug string, md *meta.Data) error {
	g.md = md
	g.spec = newSpec(appSlug)

	for _, svc := range md.Svcs {
		if err := g.addService(svc); err != nil {
			return err
		}
	}

	out, err := g.spec.MarshalJSON()
	if err != nil {
		return errors.Wrap(err, "marshal openapi spec")
	}
	buf.Write(out)

	return nil
}

func (g *Generator) addService(svc *meta.Service) error {
	for _, rpc := range svc.Rpcs {
		if err := g.addRPC(rpc); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) addRPC(rpc *meta.RPC) error {
	item := g.getOrCreatePath(rpc)

	encodings, err := encoding.DescribeRPC(g.md, rpc, &encoding.Options{})
	if err != nil {
		return errors.Wrapf(err, "describe rpc %s.%s", rpc.ServiceName, rpc.Name)
	}

	for _, reqEnc := range encodings.RequestEncoding {
		for _, m := range reqEnc.HTTPMethods {
			op, err := g.newOperationForEncoding(rpc, m, reqEnc, encodings.ResponseEncoding)
			if err != nil {
				return errors.Wrapf(err, "create operation for rpc %s.%s", rpc.ServiceName, rpc.Name)
			}
			item.SetOperation(m, op)
		}
	}

	g.spec.Paths[rpcPath(rpc)] = item
	return nil
}

func (g *Generator) getOrCreatePath(rpc *meta.RPC) *openapi3.PathItem {
	path := rpcPath(rpc)
	if existing, ok := g.spec.Paths[path]; ok {
		return existing
	}
	item := &openapi3.PathItem{}
	g.spec.Paths[path] = item
	return item
}

func (g *Generator) newOperationForEncoding(rpc *meta.RPC, method string, reqEnc *encoding.RequestEncoding, respEnc *encoding.ResponseEncoding) (*openapi3.Operation, error) {
	summary, desc := splitDoc(rpc.Doc)
	op := &openapi3.Operation{
		Summary:     summary,
		Description: desc,
		OperationID: method + ":" + rpc.ServiceName + "." + rpc.Name,
		Responses:   make(openapi3.Responses),
	}

	// Add path parameters
	for _, seg := range rpc.Path.Segments {
		if seg.Type == meta.PathSegment_LITERAL {
			continue
		}

		op.Parameters = append(op.Parameters, &openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				Name:            seg.Value,
				In:              openapi3.ParameterInPath,
				Description:     "",
				Style:           openapi3.SerializationSimple,
				Explode:         ptr(false),
				AllowEmptyValue: true,
				AllowReserved:   false,
				Deprecated:      false,
				Required:        true,
				Schema:          g.pathParamType(seg.ValueType).NewRef(),
				Example:         nil,
				Examples:        nil,
				Content:         nil,
			},
		})
	}

	// Add header parameters
	for _, param := range reqEnc.HeaderParameters {
		op.Parameters = append(op.Parameters, &openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				Name:            param.WireFormat,
				In:              openapi3.ParameterInHeader,
				Description:     markdownDoc(param.Doc),
				Style:           openapi3.SerializationSimple,
				Explode:         ptr(true),
				AllowEmptyValue: true,
				AllowReserved:   false,
				Deprecated:      false,
				Required:        false,
				Schema:          g.schemaType(param.Type),
				Example:         nil,
				Examples:        nil,
				Content:         nil,
			},
		})
	}

	// Add query parameters
	for _, param := range reqEnc.QueryParameters {
		op.Parameters = append(op.Parameters, &openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				Name:            param.WireFormat,
				In:              openapi3.ParameterInQuery,
				Description:     markdownDoc(param.Doc),
				Style:           openapi3.SerializationForm,
				Explode:         ptr(true),
				AllowEmptyValue: true,
				AllowReserved:   false,
				Deprecated:      false,
				Required:        false,
				Schema:          g.schemaType(param.Type),
				Example:         nil,
				Examples:        nil,
				Content:         nil,
			},
		})
	}

	// Add request body
	if len(reqEnc.BodyParameters) > 0 {
		op.RequestBody = &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Description: "",
				Required:    false,
				Content:     g.bodyContent(reqEnc.BodyParameters),
			},
		}
	}

	// Encode the response
	{
		resp := &openapi3.Response{
			Headers:     make(openapi3.Headers),
			Links:       nil,
			Description: ptr("Success response"),
		}

		if respEnc != nil {
			for _, param := range respEnc.HeaderParameters {
				resp.Headers[param.WireFormat] = &openapi3.HeaderRef{
					Value: &openapi3.Header{Parameter: openapi3.Parameter{
						Description:     markdownDoc(param.Doc),
						Style:           openapi3.SerializationSimple,
						Explode:         ptr(true),
						AllowEmptyValue: true,
						AllowReserved:   false,
						Deprecated:      false,
						Required:        false,
						Schema:          g.schemaType(param.Type),
						Example:         nil,
						Examples:        nil,
						Content:         nil,
					}},
				}
			}

			if len(respEnc.BodyParameters) > 0 {
				resp.Content = g.bodyContent(respEnc.BodyParameters)
			}
		}

		op.Responses["200"] = &openapi3.ResponseRef{
			Value: resp,
		}
		op.Responses["default"] = &openapi3.ResponseRef{
			Ref: "#/components/responses/APIError",
		}
	}

	return op, nil
}

func rpcPath(rpc *meta.RPC) string {
	var b strings.Builder
	for _, seg := range rpc.Path.Segments {
		b.WriteString("/")
		switch seg.Type {
		case meta.PathSegment_LITERAL:
			b.WriteString(seg.Value)
		default:
			b.WriteString("{")
			b.WriteString(seg.Value)
			b.WriteString("}")
		}
	}
	return b.String()
}

func splitDoc(doc string) (plaintextSummary, markdownDescription string) {
	firstLine, remaining := doc, ""
	if idx := strings.Index(doc, "\n"); idx >= 0 {
		firstLine = doc[:idx]
		remaining = doc[idx+1:]
	}

	return plaintextDoc(firstLine), markdownDoc(remaining)
}

func plaintextDoc(doc string) string {
	var parser comment.Parser
	var pr comment.Printer
	d := parser.Parse(doc)
	return string(pr.Text(d))
}

func markdownDoc(doc string) string {
	var parser comment.Parser
	var pr comment.Printer
	d := parser.Parse(doc)
	return string(pr.Markdown(d))
}

func ptr[T any](t T) *T {
	return &t
}

func newSpec(appSlug string) *openapi3.T {
	t := &openapi3.T{
		Components: &openapi3.Components{
			RequestBodies:   make(map[string]*openapi3.RequestBodyRef),
			Responses:       make(map[string]*openapi3.ResponseRef),
			SecuritySchemes: make(map[string]*openapi3.SecuritySchemeRef),
			Schemas:         make(map[string]*openapi3.SchemaRef),
		},
		Info: &openapi3.Info{
			Title:       fmt.Sprintf("API for %s", appSlug),
			Description: "Generated by encore",
			Version:     "1",
			Extensions: map[string]any{
				"x-logo": map[string]string{
					"url":             "https://encore.dev/assets/branding/logo/logo-black.png",
					"backgroundColor": "#EEEEE1",
					"altText":         "Encore logo",
				},
			},
		},
		OpenAPI: "3.0.0",
		Paths:   make(openapi3.Paths),
	}

	// Add the local platform server:
	t.AddServer(
		&openapi3.Server{
			URL:         "http://localhost:4000",
			Description: "Encore local dev environment",
		},
	)

	t.Components.Responses["APIError"] = &openapi3.ResponseRef{
		Value: &openapi3.Response{
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type:  openapi3.TypeObject,
							Title: "APIError",
							ExternalDocs: &openapi3.ExternalDocs{
								URL: "https://pkg.go.dev/encore.dev/beta/errs#Error",
							},
							Properties: map[string]*openapi3.SchemaRef{
								"code": {
									Value: &openapi3.Schema{
										Description: "Error code",
										Example:     "not_found",
										Type:        openapi3.TypeString,
										ExternalDocs: &openapi3.ExternalDocs{
											URL: "https://pkg.go.dev/encore.dev/beta/errs#ErrCode",
										},
									},
								},
								"message": {
									Value: &openapi3.Schema{
										Description: "Error message",
										Type:        openapi3.TypeString,
									},
								},
								"details": {
									Value: &openapi3.Schema{
										Description: "Error details",
										Type:        openapi3.TypeObject,
									},
								},
							},
						},
					},
				},
			},
			Description: ptr("Error response"),
		},
	}

	return t
}
