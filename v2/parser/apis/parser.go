package apis

import (
	"go/ast"
	"go/token"

	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/directive"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/servicestruct"
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
	Pkg            *pkginfo.Package // the package the results are for
	Endpoints      []*api.Endpoint
	AuthHandlers   []*authhandler.AuthHandler
	Middleware     []*middleware.Middleware
	ServiceStructs []*servicestruct.ServiceStruct
}

// Parse parses the given package for use of the API framework and returns
// the [ParseResult] describing the results if there's usage of the API framework
//
// If the package does not use the API framework, Parse returns nil.
func (p *Parser) Parse(pkg *pkginfo.Package) *ParseResult {
	res := &ParseResult{Pkg: pkg}
	apiFrameworkUsed := false

	for _, file := range pkg.Files {
		for _, decl := range file.AST().Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Doc == nil {
					continue
				}

				dir, doc, ok := directive.Parse(p.c.Errs, decl.Doc)
				if !ok {
					continue
				} else if dir == nil {
					continue
				}

				switch dir.Name {
				case "api":
					r := api.Parse(api.ParseData{
						Errs:   p.c.Errs,
						Schema: p.schema,
						File:   file,
						Func:   decl,
						Dir:    dir,
						Doc:    doc,
					})
					if r != nil {
						res.Endpoints = append(res.Endpoints, r)
					}
					apiFrameworkUsed = true

				case "authhandler":
					r := authhandler.Parse(authhandler.ParseData{
						Errs:   p.c.Errs,
						Schema: p.schema,
						File:   file,
						Func:   decl,
						Dir:    dir,
						Doc:    doc,
					})
					if r != nil {
						res.AuthHandlers = append(res.AuthHandlers, r)
					}
					apiFrameworkUsed = true

				case "middleware":
					r := middleware.Parse(middleware.ParseData{
						Errs:   p.c.Errs,
						Schema: p.schema,
						File:   file,
						Func:   decl,
						Dir:    dir,
						Doc:    doc,
					})
					if r != nil {
						res.Middleware = append(res.Middleware, r)
					}
					apiFrameworkUsed = true

				default:
					p.c.Errs.Add(errUnexpectedDirective(dir.Name).AtGoNode(decl))
				}

			case *ast.GenDecl:
				if decl.Tok != token.TYPE {
					continue
				} else if decl.Doc == nil {
					continue
				}

				dir, doc, ok := directive.Parse(p.c.Errs, decl.Doc)
				if !ok {
					continue
				} else if dir == nil {
					continue
				}

				switch dir.Name {
				case "service":
					r := servicestruct.Parse(servicestruct.ParseData{
						Errs:   p.c.Errs,
						Schema: p.schema,
						File:   file,
						Decl:   decl,
						Dir:    dir,
						Doc:    doc,
					})
					if r != nil {
						res.ServiceStructs = append(res.ServiceStructs, r)
					}
					apiFrameworkUsed = true

				default:
					p.c.Errs.Add(errUnexpectedDirective(dir.Name).AtGoNode(decl))
				}
			}
		}
	}

	if !apiFrameworkUsed {
		return nil
	}

	return res
}
