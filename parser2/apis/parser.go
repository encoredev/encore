package apis

import (
	"go/ast"

	"encr.dev/parser2/internal/parsectx"
	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
)

func NewParser(c *parsectx.Context, schema *schema.Parser) *Parser {
	return &Parser{
		c:      c,
		schema: schema,
	}
}

type Parser struct {
	c      *parsectx.Context
	schema *schema.Parser
}

// ParseResult describes the results of parsing a given package.
type ParseResult struct {
	RPCs []*RPC
}

func (p *Parser) Parse(pkg *pkginfo.Package) ParseResult {
	var res ParseResult

	for _, f := range pkg.Files {
		for _, decl := range f.AST().Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			dir, doc := p.parseDirectives(fd.Doc)
			switch dir := dir.(type) {
			case nil:
				continue

			case *rpcDirective:
				res.RPCs = append(res.RPCs, p.parseRPC(f, fd, dir, doc))
			}
		}
	}

	return res
}
