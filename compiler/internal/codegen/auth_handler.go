package codegen

import (
	"fmt"

	. "github.com/dave/jennifer/jen"

	"encr.dev/parser/encoding"
	"encr.dev/parser/est"
	v1 "encr.dev/proto/encore/parser/schema/v1"
)

func (b *Builder) buildAuthHandler(f *File, ah *est.AuthHandler) {
	bb := &authHandlerBuilder{
		Builder:    b,
		f:          f,
		svc:        ah.Svc,
		ah:         ah,
		paramsType: newStructDesc("p"),
	}
	bb.Write()
}

type authHandlerBuilder struct {
	*Builder
	f   *File
	svc *est.Service
	ah  *est.AuthHandler

	paramsType *structDesc
}

func (b *authHandlerBuilder) Write() {
	decodeAuth := b.renderDecodeAuth()
	authHandler := b.renderAuthHandler()
	paramDesc := b.renderStructDesc(b.AuthTypeName(), b.paramsType, true, b.ah.AuthData)

	defLoc := int(b.res.Nodes[b.svc.Root][b.ah.Func].Id)
	handler := Var().Id(b.authHandlerName(b.ah)).Op("=").Op("&").Qual("encore.dev/appruntime/api", "AuthHandlerDesc").Types(
		Id(b.AuthTypeName()),
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

func (b *authHandlerBuilder) AuthTypeName() string {
	return fmt.Sprintf("EncoreInternal_%sAuthData", b.ah.Name)
}

// renderDecodeAuth renders the DecodeAuth code as a func literal.
func (b *authHandlerBuilder) renderDecodeAuth() *Statement {
	return Func().Params(
		Id("req").Op("*").Qual("net/http", "Request"),
	).Params(Id("authData").Op("*").Id(b.AuthTypeName()), Err().Error()).BlockFunc(func(g *Group) {
		g.Id("authData").Op("=").Op("&").Id(b.AuthTypeName()).Values()

		enc, err := encoding.DescribeAuth(b.res.Meta, b.ah.Params, nil)
		if err != nil {
			b.errors.Addf(b.ah.Func.Pos(), "failed to describe request: %v", err.Error())
			return
		}
		isLegacyToken := enc.LegacyTokenFormat

		if isLegacyToken {
			fieldName := b.paramsType.AddField(other, "Token", String(), v1.Builtin_STRING)

			g.If(
				Id("auth").Op(":=").Id("req").Dot("Header").Dot("Get").Call(Lit("Authorization")),
				Id("auth").Op("!=").Lit(""),
			).Block(
				For(
					List(Id("_"), Id("prefix")).Op(":=").Range().Index(Op("...")).String().Values(Lit("Bearer "), Lit("Token ")),
				).Block(
					If(Qual("strings", "HasPrefix").Call(Id("auth"), Id("prefix"))).Block(
						If(
							Id("authData").Dot(fieldName).Op("=").Id("auth").Index(Id("len").Call(Id("prefix")).Op(":")),
							Id("authData").Dot(fieldName).Op("!=").Lit(""),
						).Block(
							Return(Id("authData"), Nil()),
						),
					),
				),
			)
			g.Return(Nil(), buildErr("Unauthenticated", "invalid auth param"))
			return
		}

		authType := b.schemaTypeToGoType(b.ah.Params)
		field := b.paramsType.AddField(payload, "Params", Op("*").Add(authType), v1.Builtin_ANY)
		g.Id("params").Op(":=").Op("&").Add(authType).Values()
		g.Id("authData").Dot(field).Op("=").Id("params")

		decoder := b.marshaller.NewPossibleInstance("dec")
		decoder.Add(CustomFunc(Options{Separator: "\n"}, func(g *Group) {
			b.decodeHeaders(g, b.ah.Func.Pos(), decoder, enc.HeaderParameters)
			b.decodeQueryString(g, b.ah.Func.Pos(), decoder, enc.QueryParameters)
		}))
		decoder.EndBlock(If(Id("dec").Dot("NonEmptyValues").Op("==").Lit(0)).Block(
			Return(Nil(), buildErr("Unauthenticated", "missing auth param")),
		))
		g.Add(decoder.Finalize(
			Return(Nil(), buildErrf("InvalidArgument", "invalid auth param: %v", Id("dec").Dot("LastError"))),
		)...)
		g.Return(Id("authData"), Nil())
	})
}

// renderAuthHandler renders the AuthHandler code as a func literal.
func (b *authHandlerBuilder) renderAuthHandler() *Statement {
	return Func().Params(
		Id("ctx").Qual("context", "Context"),
		Id("authData").Op("*").Id(b.AuthTypeName()),
	).Params(Id("info").Qual("encore.dev/appruntime/model", "AuthInfo"), Err().Error()).BlockFunc(func(g *Group) {
		threeParams := b.ah.AuthData != nil
		g.ListFunc(func(g *Group) {
			g.Id("info").Dot("UID")
			if threeParams {
				g.Id("info").Dot("UserData")
			}
			g.Err()
		}).Op("=").Qual(b.ah.Svc.Root.ImportPath, b.ah.Name).CallFunc(func(g *Group) {
			g.Id("ctx")
			for _, f := range b.paramsType.fields {
				g.Id("authData").Dot(f.fieldName)
			}
		})
		g.Return(Id("info"), Err())
	})
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
