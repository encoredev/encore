package grpcservice

import (
	"go/ast"
	"go/token"

	"google.golang.org/protobuf/reflect/protoreflect"

	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/resourcepaths"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/parser/apis/api"
)

type ServiceDesc struct {
	Errs   *perr.List
	Schema *schema.Parser
	Proto  protoreflect.ServiceDescriptor
	Pkg    *pkginfo.Package
	Decl   *schema.TypeDecl // decl is the type implementing the service
}

// ParseEndpoints parses the endpoints for the given service descriptor.
func ParseEndpoints(desc ServiceDesc) []*api.GRPCEndpoint {
	// Construct a map of method names to method descriptors so we can quickly
	// determine whether a func declaration is a candidate for being an endpoint.
	methodsByName := make(map[string]protoreflect.MethodDescriptor)
	methods := desc.Proto.Methods()
	for i := 0; i < methods.Len(); i++ {
		m := methods.Get(i)
		methodsByName[string(m.Name())] = m
	}

	structQual := desc.Decl.Info.QualifiedName()

	var endpoints []*api.GRPCEndpoint
	for _, file := range desc.Pkg.Files {
		for _, decl := range file.AST().Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv.NumFields() == 0 {
				continue
			}
			method := methodsByName[fd.Name.Name]
			if method == nil {
				continue
			}
			// We have a match on the method name.
			// Make sure the receiver type is the same as the service decl.
			funcDecl, ok := desc.Schema.ParseFuncDecl(file, fd)
			if !ok || funcDecl.Recv.Empty() {
				continue
			} else if qual := funcDecl.Recv.MustGet().Decl.Info.QualifiedName(); qual != structQual {
				// The receiver belongs to a different type; ignore it.
				continue
			}

			endpoints = append(endpoints, &api.GRPCEndpoint{
				Name:      string(method.Name()),
				FullName:  method.FullName(),
				Decl:      funcDecl,
				ProtoDesc: method,
				Path: &resourcepaths.Path{
					StartPos: token.NoPos,
					// "/path.to.Service/Method"
					Segments: []resourcepaths.Segment{
						{
							Type:      resourcepaths.Literal,
							Value:     string(desc.Proto.FullName()),
							ValueType: schema.String,
						},
						{
							Type:      resourcepaths.Literal,
							Value:     string(method.Name()),
							ValueType: schema.String,
						},
					},
				},
			})
		}
	}

	return endpoints
}
