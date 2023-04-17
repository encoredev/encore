package authhandlergen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/option"
	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen/apigenutil"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/apis/api/apienc"
	"encr.dev/v2/parser/apis/authhandler"
)

func Gen(gen *codegen.Generator, appDesc *app.Desc, ah *authhandler.AuthHandler, svcStruct option.Option[*codegen.VarDecl]) *codegen.VarDecl {
	f := gen.File(ah.Decl.File.Pkg, "authhandler")
	enc := apienc.DescribeAuth(gen.Errs, ah.Param)
	gu := gen.Util
	desc := f.VarDecl("AuthDesc", ah.Name)

	svcName := "UNKNOWN"
	svcNum := 0
	if svc, ok := appDesc.ServiceForPath(ah.Decl.File.FSPath); ok {
		svcName = svc.Name
		svcNum = svc.Num
	}

	desc.Value(Op("&").Add(apiQ("AuthHandlerDesc")).Types(
		gu.Type(ah.Param),
	).Values(Dict{
		Id("Service"): Lit(svcName),
		Id("SvcNum"):  Lit(svcNum),
		Id("DefLoc"):  Lit(gen.TraceNodes.AuthHandler()),

		Id("Endpoint"):    Lit(ah.Name),
		Id("HasAuthData"): Lit(ah.AuthData.Present()),
		Id("DecodeAuth"):  renderDecodeAuth(gen, f, ah, enc),
		Id("AuthHandler"): renderAuthHandler(gen, ah, svcStruct),
	}))

	f.Add(Func().Id("init").Params().Block(
		Qual("encore.dev/appruntime/apisdk/api", "RegisterAuthHandler").Call(
			desc.Qual(),
		),
	))

	if authData, ok := ah.AuthData.Get(); ok {
		snippet := Qual("encore.dev/appruntime/apisdk/api", "RegisterAuthDataType").
			Types(gu.Type(authData.ToType())).Call()

		if dstPkg := authData.Decl.File.Pkg; dstPkg.ImportPath == ah.Decl.File.Pkg.ImportPath {
			// It's in the same package as the auth handler; add it to the file we're already generating.
			f.Add(Func().Id("init").Params().Block(snippet))
		} else {
			// It's a different package; inject a new file.
			gen.File(dstPkg, "authdata").Add(Func().Id("init").Params().Block(snippet))
		}
	}

	return desc
}

func apiQ(name string) *Statement {
	return Qual("encore.dev/appruntime/apisdk/api", name)
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
		apigenutil.DecodeHeaders(g, Id("httpReq").Dot("Header"), Id("params"), dec, enc.HeaderParameters)
		apigenutil.DecodeQuery(g, Id("httpReq").Dot("URL").Dot("Query").Call(), Id("params"), dec, enc.QueryParameters)
		apigenutil.DecodeCookie(gen.Errs, g, Id("httpReq"), Id("params"), dec, enc.CookieParameters)

		g.If(dec.NumNonEmptyValues().Op("==").Lit(0)).Block(
			Return(gu.Zero(ah.Param), apigenutil.BuildErr("Unauthenticated", "missing auth param")),
		).Else().If(Err().Op(":=").Add(dec.Err()), Err().Op("!=").Nil()).Block(
			Return(gu.Zero(ah.Param), apigenutil.BuildErrf("InvalidArgument", "invalid auth param: %v", Err())),
		)
		g.Return(Id("params"), Nil())
	})
}

func renderAuthHandler(gen *codegen.Generator, ah *authhandler.AuthHandler, svcStruct option.Option[*codegen.VarDecl]) *Statement {
	gu := gen.Util
	return Func().Params(
		Id("ctx").Qual("context", "Context"),
		Id("params").Add(gu.Type(ah.Param)),
	).Params(Id("info").Qual("encore.dev/appruntime/exported/model", "AuthInfo"), Err().Error()).BlockFunc(func(g *Group) {
		// fnExpr is the expression for the function we want to call,
		// either just MyRPCName or svc.MyRPCName if we have a service struct.
		var fnExpr *Statement

		if ss, ok := svcStruct.Get(); ok && ah.Recv.Present() {
			g.List(Id("svc"), Id("initErr")).Op(":=").Add(ss.Qual()).Dot("Get").Call()
			g.If(Id("initErr").Op("!=").Nil()).Block(
				Return(Id("info"), Id("initErr")),
			)
			fnExpr = Id("svc").Dot(ah.Name)
		} else {
			fnExpr = Qual(ah.Decl.File.Pkg.ImportPath.String(), ah.Name)
		}

		threeParams := ah.AuthData.Present()
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
