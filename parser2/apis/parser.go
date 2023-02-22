package apis

import (
	"go/ast"
	"go/token"

	"encr.dev/parser2/apis/directive"
	"encr.dev/parser2/apis/rpc"
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
	RPCs           []*rpc.RPC
	AuthHandlers   []*AuthHandler
	Middleware     []*Middleware
	ServiceStructs []*ServiceStruct
}

func (p *Parser) Parse(pkg *pkginfo.Package) ParseResult {
	var res ParseResult
	for _, file := range pkg.Files {
		for _, decl := range file.AST().Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				dir, doc := p.parseDirectives(decl.Doc)
				switch dir := dir.(type) {

				// Parse the various directives operating on functions.
				case *directive.rpcDirective:
					res.RPCs = append(res.RPCs, p.parseRPC(file, decl, dir, doc))
				case *directive.authHandlerDirective:
					res.AuthHandlers = append(res.AuthHandlers, p.parseAuthHandler(file, decl, dir, doc))
				case *directive.middlewareDirective:
					res.Middleware = append(res.Middleware, p.parseMiddleware(file, decl, dir, doc))

				case nil:
					// do nothing
				default:
					p.c.Errs.Addf(decl.Pos(), "unexpected directive type %T on function declaration", dir)
				}

			case *ast.GenDecl:
				if decl.Tok != token.TYPE {
					continue
				}

				dir, doc := p.parseDirectives(decl.Doc)
				switch dir := dir.(type) {
				case *directive.serviceDirective:
					res.ServiceStructs = append(res.ServiceStructs, p.parseServiceStruct(file, decl, dir, doc))

				case nil:
					// do nothing
				default:
					p.c.Errs.Addf(decl.Pos(), "unexpected directive type %T on type declaration", dir)
				}
			}
		}
	}

	return res
}
