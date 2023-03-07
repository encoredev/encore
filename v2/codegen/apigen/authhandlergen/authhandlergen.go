package authhandlergen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen/apigenutil"
	"encr.dev/v2/internal/schema/schemautil"
	"encr.dev/v2/parser/apis/api/apienc"
	"encr.dev/v2/parser/apis/authhandler"
)

func Gen(gen *codegen.Generator, ah *authhandler.AuthHandler) *codegen.VarDecl {
	f := gen.File(ah.Decl.File.Pkg, "authhandler")
	enc := apienc.DescribeAuth(gen.Errs, ah.Param)
	gu := gen.Util
	desc := f.VarDecl("AuthDesc", ah.Name)

	desc.Value(Op("&").Add(apiQ("AuthHandlerDesc")).Types(
		gu.Type(ah.Param),
	).Values(Dict{
		Id("Service"): Lit("SERVICE"), // TODO
		Id("SvcNum"):  Lit(0),         // TODO
		Id("DefLoc"):  Lit(0),         // TODO

		Id("Endpoint"):    Lit(ah.Name),
		Id("HasAuthData"): Lit(ah.AuthData.IsPresent()),
		Id("DecodeAuth"):  renderDecodeAuth(gen, f, ah, enc),
		Id("AuthHandler"): renderAuthHandler(gen, f, ah, enc),
	}))

	return desc
}

func apiQ(name string) *Statement {
	return Qual("encore.dev/appruntime/api", name)
}

func renderDecodeAuth(gen *codegen.Generator, f *codegen.File, ah *authhandler.AuthHandler, enc *apienc.AuthEncoding) *Statement {
	gu := gen.Util
	return Func().Params(
		Id("httpReq").Op("*").Qual("net/http", "Request"),
	).Params(Id("params").Add(gu.Type(ah.Param)), Err().Error()).BlockFunc(func(g *Group) {
		// Initialize params if it's a pointer so we're always dealing with a valid value
		if schemautil.IsPointer(ah.Param) {
			g.Id("params").Op("=").Add(gu.Initialize(ah.Param))
		}

		isLegacyToken := enc.LegacyTokenFormat
		if isLegacyToken {
			g.If(
				Id("auth").Op(":=").Id("httpReq").Dot("Header").Dot("Get").Call(Lit("Authorization")),
				Id("auth").Op("!=").Lit(""),
			).Block(
				For(
					List(Id("_"), Id("prefix")).Op(":=").Range().Index(Op("...")).String().Values(Lit("Bearer "), Lit("Token ")),
				).Block(
					If(Qual("strings", "HasPrefix").Call(Id("auth"), Id("prefix"))).Block(
						If(
							Id("params").Op("=").Id("auth").Index(Id("len").Call(Id("prefix")).Op(":")),
							Id("params").Op("!=").Lit(""),
						).Block(
							Return(Id("params"), Nil()),
						),
					),
				),
			)
			g.Return(gu.Zero(ah.Param), apigenutil.BuildErr("Unauthenticated", "invalid auth param"))
			return
		}

		dec := gu.NewTypeUnmarshaller("dec")
		g.Add(dec.Init())
		apigenutil.DecodeHeaders(g, Id("httpReq"), Id("params"), dec, enc.HeaderParameters)
		apigenutil.DecodeQuery(g, Id("httpReq"), Id("params"), dec, enc.QueryParameters)

		g.If(dec.NumNonEmptyValues().Op("==").Lit(0)).Block(
			Return(gu.Zero(ah.Param), apigenutil.BuildErr("Unauthenticated", "missing auth param")),
		).Else().If(Err().Op(":=").Add(dec.Err()), Err().Op("!=").Nil()).Block(
			Return(gu.Zero(ah.Param), apigenutil.BuildErrf("InvalidArgument", "invalid auth param: %v", Err())),
		)
		g.Return(Id("params"), Nil())
	})
}

func renderAuthHandler(gen *codegen.Generator, f *codegen.File, ah *authhandler.AuthHandler, enc *apienc.AuthEncoding) *Statement {
	gu := gen.Util
	return Func().Params(
		Id("ctx").Qual("context", "Context"),
		Id("params").Add(gu.Type(ah.Param)),
	).Params(Id("info").Qual("encore.dev/appruntime/model", "AuthInfo"), Err().Error()).BlockFunc(func(g *Group) {
		// fnExpr is the expression for the function we want to call,
		// either just MyRPCName or svc.MyRPCName if we have a service struct.
		var fnExpr *Statement

		// TODO(andre) Implement support for service structs
		// If we have a service struct, initialize it first.
		//group := b.ah.SvcStruct
		//if group != nil {
		//	ss := b.ah.Svc.Struct
		//	g.List(Id("svc"), Id("initErr")).Op(":=").Qual(b.ah.Svc.Root.ImportPath, b.serviceStructName(ss)).Dot("Get").Call()
		//	g.If(Id("initErr").Op("!=").Nil()).Block(
		//		Return(Id("info"), Id("initErr")),
		//	)
		//	fnExpr = Id("svc").Dot(b.ah.Name)
		//} else {
		//	fnExpr = Qual(b.ah.Svc.Root.ImportPath, b.ah.Name)
		//}
		fnExpr = Qual(ah.Decl.File.Pkg.ImportPath.String(), ah.Name)

		threeParams := ah.AuthData.IsPresent()
		g.ListFunc(func(g *Group) {
			g.Id("info").Dot("UID")
			if threeParams {
				g.Id("info").Dot("UserData")
			}
			g.Err()
		}).Op("=").Add(fnExpr).Call(Id("ctx"), Id("params"))
		g.Return(Id("info"), Err())
	})
}
