package codegen

import (
	"fmt"

	. "github.com/dave/jennifer/jen"

	"encr.dev/parser/encoding"
	"encr.dev/parser/est"
)

func (b *Builder) buildAuthHandler(f *File, ah *est.AuthHandler) {
	enc, err := encoding.DescribeAuth(b.res.Meta, ah.Params, nil)
	if err != nil {
		b.errors.Addf(ah.Func.Pos(), "failed to describe auth handler: %v", err.Error())
		return
	}

	bb := &authHandlerBuilder{
		Builder: b,
		f:       f,
		svc:     ah.Svc,
		ah:      ah,
		enc:     enc,
	}
	bb.Write()
}

type authHandlerBuilder struct {
	*Builder
	f   *File
	svc *est.Service
	ah  *est.AuthHandler
	enc *encoding.AuthEncoding
}

func (b *authHandlerBuilder) Write() {
	decodeAuth := b.renderDecodeAuth()
	authHandler := b.renderAuthHandler()
	paramDesc := b.renderAuthHandlerStructDesc()

	defLoc := int(b.res.Nodes[b.svc.Root][b.ah.Func].Id)
	handler := Var().Id(b.authHandlerName(b.ah)).Op("=").Op("&").Qual("encore.dev/appruntime/api", "AuthHandlerDesc").Types(
		b.ParamsType(),
	).Custom(Options{
		Open:      "{",
		Close:     "}",
		Separator: ",",
		Multi:     true,
	},
		Id("Service").Op(":").Lit(b.svc.Name),
		Id("Endpoint").Op(":").Lit(b.ah.Name),
		Id("DefLoc").Op(":").Lit(defLoc),
		Id("HasAuthData").Op(":").Lit(b.ah.AuthData != nil),
		Id("DecodeAuth").Op(":").Add(decodeAuth),
		Id("AuthHandler").Op(":").Add(authHandler),
		Id("SerializeParams").Op(":").Add(paramDesc.Serialize),
	)

	for _, part := range [...]Code{
		paramDesc.TypeDecl,
		handler,
	} {
		b.f.Add(part)
		b.f.Line()
	}
}

func (b *Builder) authHandlerName(ah *est.AuthHandler) string {
	return fmt.Sprintf("EncoreInternal_%sAuthHandler", ah.Name)
}

func (b *authHandlerBuilder) ParamsIsPtr() bool {
	if b.enc.LegacyTokenFormat {
		return false
	}
	return true
}

func (b *authHandlerBuilder) ParamsType() *Statement {
	if b.enc.LegacyTokenFormat {
		return String()
	}
	return Op("*").Id(b.ParamsTypeName())
}

func (b *authHandlerBuilder) ParamsZeroValue() *Statement {
	if b.enc.LegacyTokenFormat {
		return Lit("")
	} else if b.ParamsIsPtr() {
		return Nil()
	} else {
		return b.ParamsType().Values()
	}
}

func (b *authHandlerBuilder) ParamsTypeName() string {
	return fmt.Sprintf("EncoreInternal_%sAuthParams", b.ah.Name)
}

// renderDecodeAuth renders the DecodeAuth code as a func literal.
func (b *authHandlerBuilder) renderDecodeAuth() *Statement {
	return Func().Params(
		Id("req").Op("*").Qual("net/http", "Request"),
	).Params(Id("params").Add(b.ParamsType()), Err().Error()).BlockFunc(func(g *Group) {
		// Initialize params if it's a pointer so we're always dealing with a valid value
		if b.ParamsIsPtr() {
			g.Id("params").Op("=").Op("&").Id(b.ParamsTypeName()).Values()
		}

		isLegacyToken := b.enc.LegacyTokenFormat
		if isLegacyToken {
			g.If(
				Id("auth").Op(":=").Id("req").Dot("Header").Dot("Get").Call(Lit("Authorization")),
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
			g.Return(b.ParamsZeroValue(), buildErr("Unauthenticated", "invalid auth param"))
			return
		}

		decoder := b.marshaller.NewPossibleInstance("dec")
		decoder.Add(CustomFunc(Options{Separator: "\n"}, func(g *Group) {
			b.decodeHeaders(g, b.ah.Func.Pos(), decoder, b.enc.HeaderParameters)
			b.decodeQueryString(g, b.ah.Func.Pos(), decoder, b.enc.QueryParameters)
		}))
		decoder.EndBlock(If(Id("dec").Dot("NonEmptyValues").Op("==").Lit(0)).Block(
			Return(b.ParamsZeroValue(), buildErr("Unauthenticated", "missing auth param")),
		))
		g.Add(decoder.Finalize(
			Return(b.ParamsZeroValue(), buildErrf("InvalidArgument", "invalid auth param: %v", Id("dec").Dot("LastError"))),
		)...)
		g.Return(Id("params"), Nil())
	})
}

// renderAuthHandler renders the AuthHandler code as a func literal.
func (b *authHandlerBuilder) renderAuthHandler() *Statement {
	return Func().Params(
		Id("ctx").Qual("context", "Context"),
		Id("params").Add(b.ParamsType()),
	).Params(Id("info").Qual("encore.dev/appruntime/model", "AuthInfo"), Err().Error()).BlockFunc(func(g *Group) {
		// fnExpr is the expression for the function we want to call,
		// either just MyRPCName or svc.MyRPCName if we have a service struct.
		var fnExpr *Statement

		// If we have a service struct, initialize it first.
		group := b.ah.SvcStruct
		if group != nil {
			ss := b.ah.Svc.Struct
			g.List(Id("svc"), Id("initErr")).Op(":=").Qual(b.ah.Svc.Root.ImportPath, b.serviceStructName(ss)).Dot("Get").Call()
			g.If(Id("initErr").Op("!=").Nil()).Block(
				Return(Id("info"), Id("initErr")),
			)
			fnExpr = Id("svc").Dot(b.ah.Name)
		} else {
			fnExpr = Qual(b.ah.Svc.Root.ImportPath, b.ah.Name)
		}

		threeParams := b.ah.AuthData != nil
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

func (b *authHandlerBuilder) renderAuthHandlerStructDesc() structCodegen {
	ah := b.ah
	var result structCodegen

	result.TypeDecl = Type().Id(b.ParamsTypeName()).Op("=").Do(func(s *Statement) {
		if b.enc.LegacyTokenFormat {
			s.String()
		} else {
			s.Add(b.schemaTypeToGoType(derefPointer(ah.Params)))
		}
	})

	result.Serialize = Func().Params(
		Id("json").Qual("github.com/json-iterator/go", "API"),
		Id("params").Add(b.ParamsType()),
	).Params(
		Index().Index().Byte(),
		Error(),
	).BlockFunc(func(g *Group) {
		g.List(Id("v"), Err()).Op(":=").Id("json").Dot("Marshal").Call(Id("params"))
		g.If(Err().Op("!=").Nil()).Block(Return(Nil(), Err()))
		g.Return(Index().Index().Byte().Values(Id("v")), Nil())
	})

	result.Clone = Func().Params(
		Id("params").Add(b.ParamsType()),
	).Params(
		b.ParamsType(),
		Error(),
	).BlockFunc(func(g *Group) {
		// We could optimize the clone operation if there are no reference types (pointers, maps, slices)
		// in the struct. For now, simply serialize it as JSON and back.
		g.Var().Id("clone").Id(b.ParamsTypeName())

		g.List(Id("bytes"), Id("err")).Op(":=").Qual("github.com/json-iterator/go", "ConfigDefault").Dot("Marshal").Call(Id("resp"))
		g.If(Err().Op("==").Nil()).Block(
			Err().Op("=").Qual("github.com/json-iterator/go", "ConfigDefault").Dot("Unmarshal").Call(Id("bytes"), Op("&").Id("clone")),
		)

		retExpr := Id("clone")
		if b.ParamsIsPtr() {
			retExpr = Op("&").Add(retExpr)
		}
		g.Return(retExpr, Err())
	})

	return result
}

func buildErr(code, msg string) *Statement {
	p := "encore.dev/beta/errs"
	return Qual(p, "B").Call().Dot("Code").Call(Qual(p, code)).Dot("Msg").Call(Lit(msg)).Dot("Err").Call()
}

func buildErrf(code, format string, args ...Code) *Statement {
	p := "encore.dev/beta/errs"
	args = append([]Code{Lit(format)}, args...)
	return Qual(p, "B").Call().Dot("Code").Call(Qual(p, code)).Dot("Msgf").Call(args...).Dot("Err").Call()
}
