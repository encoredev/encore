package apis

import (
	"go/ast"
	"go/token"

	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/internal/directive"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/servicestruct"
	"encr.dev/v2/parser/apis/tracedfunc"
	"encr.dev/v2/parser/resource/resourceparser"
)

var Parser = &resourceparser.Parser{
	Name: "APIs",

	InterestingImports: resourceparser.RunAlways,
	Run: func(p *resourceparser.Pass) {
		for _, file := range p.Pkg.Files {
			for _, decl := range file.AST().Decls {
				switch decl := decl.(type) {
				case *ast.FuncDecl:
					if decl.Doc == nil {
						continue
					}

					dir, doc, ok := directive.Parse(p.Errs, decl.Doc)
					if !ok {
						continue
					} else if dir == nil {
						continue
					}

					switch dir.Name {
					case "api":
						ep := api.Parse(api.ParseData{
							Errs:   p.Errs,
							Schema: p.SchemaParser,
							File:   file,
							Func:   decl,
							Dir:    dir,
							Doc:    doc,
						})

						if ep != nil {
							p.RegisterResource(ep)
							// We unconditionally register a package-level bind here,
							// even if the endpoint is defined on a service struct.
							//
							// This is the case because we generate a package-level
							// wrapper function that forwards to the service struct
							// method in that case.
							p.AddNamedBind(file, ep.Decl.AST.Name, ep)
						}

					case "authhandler":
						ah := authhandler.Parse(authhandler.ParseData{
							Errs:   p.Errs,
							Schema: p.SchemaParser,
							File:   file,
							Func:   decl,
							Dir:    dir,
							Doc:    doc,
						})
						if ah != nil {
							p.RegisterResource(ah)
							if ah.Recv.Empty() {
								p.AddNamedBind(file, ah.Decl.AST.Name, ah)
							}
						}

					case "middleware":
						mw := middleware.Parse(middleware.ParseData{
							Errs:   p.Errs,
							Schema: p.SchemaParser,
							File:   file,
							Func:   decl,
							Dir:    dir,
							Doc:    doc,
						})

						if mw != nil {
							p.RegisterResource(mw)
							if mw.Recv.Empty() {
								p.AddNamedBind(file, mw.Decl.AST.Name, mw)
							}
						}

					case "trace":
						tf := tracedfunc.Parse(tracedfunc.ParseData{
							Errs:   p.Errs,
							Schema: p.SchemaParser,
							File:   file,
							Func:   decl,
							Dir:    dir,
							Doc:    doc,
						})

						if tf != nil {
							p.RegisterResource(tf)
							// We don't bind this function because we don't need to know
							// anything else, apart from the fact it exists and is traced.
						}

					default:
						p.Errs.Add(errUnexpectedDirective(dir.Name).AtGoNode(decl))
					}

				case *ast.GenDecl:
					if decl.Tok != token.TYPE {
						continue
					} else if decl.Doc == nil {
						continue
					}

					dir, doc, ok := directive.Parse(p.Errs, decl.Doc)
					if !ok {
						continue
					} else if dir == nil {
						continue
					}

					switch dir.Name {
					case "service":
						ss := servicestruct.Parse(servicestruct.ParseData{
							Errs:   p.Errs,
							Schema: p.SchemaParser,
							File:   file,
							Decl:   decl,
							Dir:    dir,
							Doc:    doc,
						})

						if ss != nil {
							p.RegisterResource(ss)
							p.AddNamedBind(file, ss.Decl.AST.Name, ss)
						}

					default:
						p.Errs.Add(errUnexpectedDirective(dir.Name).AtGoNode(decl))
					}
				}
			}
		}
	},
}
