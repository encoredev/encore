package openapi

import (
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

// TODO spec.{Info,Servers,Security}

type Generator struct {
	spec      *openapi3.T
	md        *meta.Data
	seenDecls map[string]uint32
}

func New(md *meta.Data, version GenVersion) (*Generator, error) {
	if version > LatestVersion {
		return nil, errors.Errorf("unknown openapi generator version %d", version)
	}

	return &Generator{
		spec:      newSpec(),
		md:        md,
		seenDecls: make(map[string]uint32),
	}, nil
}

func (g *Generator) Generate(buf)

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
		op, err := g.newOperationForEncoding(rpc, reqEnc, encodings.ResponseEncoding)
		if err != nil {
			return errors.Wrapf(err, "create operation for rpc %s.%s", rpc.ServiceName, rpc.Name)
		}
		for _, m := range reqEnc.HTTPMethods {
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

func (g *Generator) newOperationForEncoding(rpc *meta.RPC, reqEnc *encoding.RequestEncoding, respEnc *encoding.ResponseEncoding) (*openapi3.Operation, error) {
	summary, desc := splitDoc(rpc.Doc)
	op := &openapi3.Operation{
		Summary:     summary,
		Description: desc,
		OperationID: rpc.ServiceName + "." + rpc.Name,
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
				Required:        false,
				Schema:          nil, // TODO
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
			Headers: make(openapi3.Headers),
			Links:   nil,
		}
		for _, param := range respEnc.HeaderParameters {
			resp.Headers[param.Name] = &openapi3.HeaderRef{
				Value: &openapi3.Header{Parameter: openapi3.Parameter{
					Name:            param.WireFormat,
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

		op.Responses["200"] = &openapi3.ResponseRef{
			Value: resp,
		}
		// TODO error response
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

func newSpec() *openapi3.T {
	t := &openapi3.T{
		Components: &openapi3.Components{
			RequestBodies:   make(map[string]*openapi3.RequestBodyRef),
			Responses:       make(map[string]*openapi3.ResponseRef),
			SecuritySchemes: make(map[string]*openapi3.SecuritySchemeRef),
			Schemas:         make(map[string]*openapi3.SchemaRef),
		},
		Info: &openapi3.Info{
			Title:       "Encore API",
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
	t.Components.Responses["EncoreAPIError"] = &openapi3.ResponseRef{
		Value: &openapi3.Response{
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type:  openapi3.TypeObject,
							Title: "EncoreAPIError",
							Properties: map[string]*openapi3.SchemaRef{
								"Id": {
									Value: &openapi3.Schema{
										Description: "Error ID",
										Type:        openapi3.TypeString,
									},
								},
								"Code": {
									Value: &openapi3.Schema{
										Description: "Error code",
										Example:     500,
										Type:        openapi3.TypeString,
									},
								},
								"Detail": {
									Value: &openapi3.Schema{
										Description: "Error detail",
										Example:     "service not found",
										Type:        openapi3.TypeString,
									},
								},
								"Status": {
									Value: &openapi3.Schema{
										Description: "Error status message",
										Example:     "Internal Server Error",
										Type:        openapi3.TypeString,
									},
								},
							},
						},
					},
				},
			},
			Description: ptr("Error from the Micro API"),
		},
	}
}
