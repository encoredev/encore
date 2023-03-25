package endpointgen

import (
	"go/ast"

	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/apis/api"
)

func rewriteAPICalls(gen *codegen.Generator, parse *parser.Result, svc *app.Service, ep *api.Endpoint, desc *handlerDesc) {
	var fd *codegen.FuncDecl

	for _, u := range parse.Usages(ep) {
		if call, ok := u.(*api.CallUsage); ok {
			// Generate the wrapper the first time it's needed.
			if fd == nil {
				fd = genCallWrapper(gen, svc, ep, desc)
			}

			rw := gen.Rewrite(call.File)
			if sel, ok := call.Call.Fun.(*ast.SelectorExpr); ok {
				rw.ReplaceNode(sel.Sel, []byte(fd.Name()))
			} else {
				rw.ReplaceNode(call.Call.Fun, []byte(fd.Name()))
			}
		}
	}
}

func genCallWrapper(gen *codegen.Generator, svc *app.Service, ep *api.Endpoint, handler *handlerDesc) *codegen.FuncDecl {
	gu := gen.Util
	fw := svc.Framework.MustGet()
	f := gen.File(fw.RootPkg, "apicalls")
	fd := f.FuncDecl(ep.Name)

	type param struct {
		name string
		typ  *Statement
	}

	var params []param
	addParam := func(name string, typ *Statement) {
		params = append(params, param{name, typ})
		fd.Params(Id(name).Add(typ.Clone()))
	}

	// Generate parameters
	fd.Params(Id("ctx").Qual("context", "Context"))
	for idx, param := range ep.Path.Params() {
		addParam(handler.req.pathParamFieldName(idx), gu.Builtin(param.Pos(), param.ValueType))
	}
	if ep.Request != nil {
		addParam(handler.req.reqDataPayloadName(), gu.Type(ep.Request))
	}

	// Generate results
	if ep.Response != nil {
		fd.Results(gu.Type(ep.Response))
	}
	fd.Results(Error())

	// Generate body
	fd.BodyFunc(func(g *Group) {
		g.ListFunc(func(g *Group) {
			if ep.Response != nil {
				g.Id("resp")
			} else {
				g.Id("_")
			}
			g.Err()
		}).Op(":=").Id(handler.desc.Name()).Dot("Call").CallFunc(func(g *Group) {
			g.Add(apiQ("NewCallContext")).Call(Id("ctx"))
			g.Op("&").Id(handler.req.TypeName()).Values(DictFunc(func(d Dict) {
				for _, p := range params {
					d[Id(p.name)] = Id(p.name)
				}
			}))
		})
		g.If(Err().Op("!=").Nil()).BlockFunc(func(g *Group) {
			if ep.Response != nil {
				g.Return(gu.Zero(ep.Response), Err())
			} else {
				g.Return(Err())
			}
		})
		if ep.Response != nil {
			g.Return(Id("resp"), Nil())
		} else {
			g.Return(Nil())
		}
	})

	return fd
}
