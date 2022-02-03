package codegen

import (
	"fmt"
	"strconv"

	"encr.dev/parser/est"
	"encr.dev/parser/paths"
	schema "encr.dev/proto/encore/parser/schema/v1"

	. "github.com/dave/jennifer/jen" // for code gen
)

func (b *Builder) Wrappers(pkg *est.Package, wrappers []*est.RPC) *File {
	f := NewFilePathName(pkg.ImportPath, pkg.Name)
	f.ImportNames(importNames)

	for _, p := range b.res.App.Packages {
		f.ImportName(p.ImportPath, p.Name)
	}

	for _, rpc := range wrappers {
		f.Add(b.buildRPCWrapper(f, rpc))
		f.Line()
	}

	return f
}

func (b *Builder) buildRPCWrapper(f *File, rpc *est.RPC) *Statement {
	segs := make([]paths.Segment, 0, len(rpc.Path.Segments))
	for _, s := range rpc.Path.Segments {
		if s.Type != paths.Literal {
			segs = append(segs, s)
		}
	}

	numParams := 0
	return Func().Id("__encore_" + rpc.Svc.Name + "_" + rpc.Name).ParamsFunc(func(g *Group) {
		g.List(Id("callExprIdx"), Id("endpointExprIdx")).Int32()
		g.Id("ctx").Qual("context", "Context")
		for _, p := range segs {
			g.Id("p" + strconv.Itoa(numParams)).Add(b.builtinType(p.ValueType))
			numParams++
		}
		if rpc.Request != nil {
			g.Id("p" + strconv.Itoa(numParams)).Add(b.namedType(f, rpc.Request))
			numParams++
		}
	}).ParamsFunc(func(g *Group) {
		if rpc.Response != nil {
			g.Id("resp").Add(b.namedType(f, rpc.Response))
		}
		g.Err().Error()
	}).BlockFunc(func(g *Group) {
		if numParams > 0 {
			g.List(Id("inputs"), Err()).Op(":=").Qual("encore.dev/runtime", "SerializeInputs").CallFunc(func(g *Group) {
				for i := 0; i < numParams; i++ {
					g.Id("p" + strconv.Itoa(i))
				}
			})
			g.If(Err().Op("!=").Nil()).Block(Return())
		} else {
			g.Var().Id("inputs").Index().Index().Byte()
		}
		g.List(Id("call"), Err()).Op(":=").Qual("encore.dev/runtime", "BeginCall").Call(Qual("encore.dev/runtime", "CallParams").Values(Dict{
			Id("Service"):         Lit(rpc.Svc.Name),
			Id("Endpoint"):        Lit(rpc.Name),
			Id("CallExprIdx"):     Id("callExprIdx"),
			Id("EndpointExprIdx"): Id("endpointExprIdx"),
		}))
		g.If(Err().Op("!=").Nil()).Block(Return())
		g.Line()
		g.Comment("Run the request in a different goroutine")
		g.Var().Id("response").Struct(
			Id("data").Index().Index().Byte(),
			Err().Error(),
		)
		g.Id("done").Op(":=").Make(Chan().Struct())
		g.Go().Func().Params().BlockFunc(func(g *Group) {
			g.Defer().Close(Id("done"))
			requireAuth := False()
			if rpc.Access == est.Auth {
				requireAuth = True()
			}
			g.Err().Op(":=").Id("call").Dot("BeginReq").Call(Id("ctx"), Qual("encore.dev/runtime", "RequestData").Values(Dict{
				Id("Type"):            Qual("encore.dev/runtime", "RPCCall"),
				Id("Service"):         Lit(rpc.Svc.Name),
				Id("Endpoint"):        Lit(rpc.Name),
				Id("CallExprIdx"):     Id("callExprIdx"),
				Id("EndpointExprIdx"): Id("endpointExprIdx"),
				Id("Inputs"):          Id("inputs"),
				Id("RequireAuth"):     requireAuth,
			}))
			g.If().Err().Op("!=").Nil().Block(
				Id("response").Dot("err").Op("=").Err(),
				Return(),
			)
			g.Defer().Func().Params().Block(
				If(Id("err2").Op(":=").Recover(), Id("err2").Op("!=").Nil()).Block(
					Id("response").Dot("err").Op("=").Add(buildErrf("Internal", "panic handling request: %v", Id("err2"))),
					Id("call").Dot("FinishReq").Call(Nil(), Id("response").Dot("err")),
				),
			).Call()
			g.Line()

			if numParams > 0 {
				g.Var().DefsFunc(func(g *Group) {
					// TODO(eandre) we could do a smarter job here of avoiding needless copies
					// for immutable types like primitives.
					for i := 0; i < numParams; i++ {
						if i < len(segs) {
							g.Id("r" + strconv.Itoa(i)).Add(b.builtinType(segs[i].ValueType))
						} else {
							g.Id("r" + strconv.Itoa(i)).Add(b.namedType(f, rpc.Request))
						}
					}
				})
				g.If(Id("response").Dot("err").Op("=").Qual("encore.dev/runtime", "CopyInputs").Call(Id("inputs"), Index().Interface().ValuesFunc(func(g *Group) {
					for i := 0; i < numParams; i++ {
						g.Op("&").Id("r" + strconv.Itoa(i))
					}
				})), Id("response").Dot("err").Op("!=").Nil()).Block(
					Id("call").Dot("FinishReq").Call(Nil(), Id("response").Dot("err")),
					Return(),
				)
			}
			g.Line()

			g.ListFunc(func(g *Group) {
				if rpc.Response != nil {
					g.Id("rpcResp")
				}
				g.Id("rpcErr")
			}).Op(":=").Qual(rpc.Svc.Root.ImportPath, rpc.Name).CallFunc(func(g *Group) {
				g.Id("ctx")
				for i := 0; i < numParams; i++ {
					g.Id("r" + strconv.Itoa(i))
				}
			})
			if rpc.Response != nil {
				g.List(Id("response").Dot("data"), Id("_")).Op("=").Qual("encore.dev/runtime", "SerializeInputs").Call(Id("rpcResp"))
			}
			g.If(Id("rpcErr").Op("!=").Nil()).Block(
				Id("call").Dot("FinishReq").Call(Nil(), Id("rpcErr")),
				Id("response").Dot("err").Op("=").Qual("encore.dev/beta/errs", "RoundTrip").Call(Id("rpcErr")),
			).Else().Block(
				Id("call").Dot("FinishReq").Call(Id("response").Dot("data"), Nil()),
			)
		}).Call()
		g.Op("<-").Id("done")
		g.Line()

		g.Id("call").Dot("Finish").Call(Id("response").Dot("err"))
		if rpc.Response != nil {
			g.If(Id("response").Dot("data").Op("!=").Nil()).Block(
				Id("_").Op("=").Qual("encore.dev/runtime", "CopyInputs").Call(Id("response").Dot("data"), Index().Interface().Values(Op("&").Id("resp"))),
			)
			g.Return(Id("resp"), Id("response").Dot("err"))
		} else {
			g.Return(Id("response").Dot("err"))
		}
	})
}

func (b *Builder) builtinType(t schema.Builtin) *Statement {
	switch t {
	case schema.Builtin_STRING:
		return String()
	case schema.Builtin_BOOL:
		return Bool()
	case schema.Builtin_INT8:
		return Int8()
	case schema.Builtin_INT16:
		return Int16()
	case schema.Builtin_INT32:
		return Int32()
	case schema.Builtin_INT64:
		return Int64()
	case schema.Builtin_INT:
		return Int()
	case schema.Builtin_UINT8:
		return Uint8()
	case schema.Builtin_UINT16:
		return Uint16()
	case schema.Builtin_UINT32:
		return Uint32()
	case schema.Builtin_UINT64:
		return Uint64()
	case schema.Builtin_UINT:
		return Uint()
	case schema.Builtin_UUID:
		return Qual("encore.dev/types/uuid", "UUID")
	default:
		panic(fmt.Sprintf("unexpected builtin type %v", t))
	}
}

func (b *Builder) namedType(f *File, param *est.Param) *Statement {
	if named := param.Type.GetNamed(); named != nil {
		decl := b.res.App.Decls[named.Id]
		f.ImportName(decl.Loc.PkgPath, decl.Loc.PkgName)
	}

	return b.typeName(param, false)
}
