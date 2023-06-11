package servicestruct

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/reflect/protoreflect"

	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/protoparse"
	"encr.dev/v2/parser/apis/internal/directive"
)

// parseGRPCDirective parses and validates the gRPC path directive.
func parseGRPCDirective(ctx context.Context, errs *perr.List, proto *protoparse.Parser, f directive.Field) (ok bool) {
	astNode := f // directive.Field implements ast.Node
	grpcPath := f.Value
	// Two ways of referencing a service:
	// - "path/to/my.proto:ServiceName"
	// - "path.to.my.ServiceName"

	var (
		filePath string
		svcName  protoreflect.Name
	)
	if idx := strings.LastIndexByte(grpcPath, ':'); idx >= 0 {
		filePath = grpcPath[:idx]
		svcName = protoreflect.Name(grpcPath[idx+1:])
		if !svcName.IsValid() {
			errs.Add(errInvalidGRPCName(f.Value).AtGoNode(astNode))
			return false
		} else if !filepath.IsLocal(f.Value) {
			errs.Add(errNonLocalGRPCPath(f.Value).AtGoNode(astNode))
			return false
		}
	} else {
		fullName := protoreflect.FullName(grpcPath)
		if !fullName.IsValid() {
			errs.Add(errInvalidGRPCName(f.Value).AtGoNode(astNode))
			return false
		}

		pkgpath := fullName.Parent()
		svcName = fullName.Name()
		if pkgpath == "" {
			// If there's no pkgpath we got a bare "Service" path, without a package name.
			errs.Add(errInvalidGRPCName(f.Value).AtGoNode(astNode))
			return false
		}
		filePath = strings.ReplaceAll(string(pkgpath), ".", "/") + ".proto"
	}

	file := proto.ParseFile(ctx, astNode, filePath)
	svc := file.Services().ByName(svcName)

	if svc == nil {
		errs.Add(errGRPCServiceNotFound(string(svcName), filePath).AtGoNode(astNode))
		return false
	}

	methods := svc.Methods()
	for i := 0; i < methods.Len(); i++ {
		m := methods.Get(i)
		log.Printf("service %s: method %s", svc.FullName(), m.Name())
	}
	return true
}
