package protoparse

import (
	"context"
	"go/ast"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/perr"
)

func NewParser(errs *perr.List, protoRoots []paths.FS) *Parser {
	c := protocompile.Compiler{
		// TODO introduce a caching resolver so we don't have to recompile shared protos
		// multiple times.
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: fns.Map(protoRoots, paths.FS.ToIO),
		}),
		// TODO(andre) Add a custom reporter
		Reporter:       nil,
		SourceInfoMode: protocompile.SourceInfoExtraComments,
		RetainASTs:     false,
	}
	return &Parser{errs: errs, c: c}
}

type Parser struct {
	errs *perr.List
	c    protocompile.Compiler
}

// ParseFile parses a single protobuf file ("path/to/file.proto").
// The filename must be relative to one of the proto roots.
// It bails out if the file is not found or cannot be parsed.
func (p *Parser) ParseFile(ctx context.Context, srcNode ast.Node, filepath string) linker.File {
	return p.parseFile(ctx, srcNode, filepath)
}

func (p *Parser) parseFile(ctx context.Context, srcNode ast.Node, file string) linker.File {
	result, err := p.c.Compile(ctx, file)
	if err != nil {
		p.errs.Assert(errInvalidProtoFile(file).AtGoNode(srcNode))
	}

	return result[0] // guaranteed to be present and correct since we only compile one file
}
