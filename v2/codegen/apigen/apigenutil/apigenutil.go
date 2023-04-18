package apigenutil

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/codegen/internal/genutil"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/apis/api/apienc"
)

// DecodeHeaders generates code for decoding HTTP headers from the http request
// given by httpReqExpr and storing the result into the params given by paramsExpr.
func DecodeHeaders(g *Group, httpReqExpr, paramExpr *Statement, dec *genutil.TypeUnmarshaller, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}
	g.Comment("Decode headers")
	g.Id("h").Op(":=").Add(httpReqExpr.Clone()).Dot("Header")
	for _, f := range params {
		singleValExpr := Id("h").Dot("Get").Call(Lit(f.WireName))
		listValExpr := Id("h").Dot("Values").Call(Lit(f.WireName))
		decodeExpr := dec.UnmarshalSingleOrList(f.Type, f.WireName, singleValExpr, listValExpr, false)
		g.Add(paramExpr.Clone()).Dot(f.SrcName).Op("=").Add(decodeExpr)
	}
	g.Line()
}

// DecodeQuery is like DecodeHeaders but for query strings.
func DecodeQuery(g *Group, httpReqExpr, paramExpr *Statement, dec *genutil.TypeUnmarshaller, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}
	g.Comment("Decode query string")
	g.Id("qs").Op(":=").Add(httpReqExpr.Clone()).Dot("URL").Dot("Query").Call()

	for _, f := range params {
		singleValExpr := Id("qs").Dot("Get").Call(Lit(f.WireName))
		listValExpr := Id("qs").Index(Lit(f.WireName))
		decodeExpr := dec.UnmarshalSingleOrList(f.Type, f.WireName, singleValExpr, listValExpr, false)
		g.Add(paramExpr.Clone()).Dot(f.SrcName).Op("=").Add(decodeExpr)
	}
	g.Line()
}

// DecodeCookie is like DecodeHeaders but for cookies.
func DecodeCookie(errs *perr.List, g *Group, httpReqExpr, paramExpr *Statement, dec *genutil.TypeUnmarshaller, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}
	g.Comment("Decode cookies")

	cookieType := pkginfo.Q("net/http", "Cookie")

	for _, f := range params {
		g.If(List(Id("c"), Id("_")).Op(":=").Add(httpReqExpr.Clone()).Dot("Cookie").Call(Lit(f.WireName)).Op(";").Id("c").Op("!=").Nil()).BlockFunc(func(g *Group) {
			// Cookies can either be a builtin or a *http.Cookie.
			if builtin, ok := f.Type.(schema.BuiltinType); ok {
				decodeExpr := dec.UnmarshalBuiltin(builtin.Kind, f.WireName, Id("c").Dot("Value"), false)
				g.Add(paramExpr.Clone()).Dot(f.SrcName).Op("=").Add(decodeExpr)
			} else if info, ok := schemautil.DerefNamedInfo(f.Type, true); ok && info.QualifiedName() == cookieType {
				g.Add(paramExpr.Clone()).Dot(f.SrcName).Op("=").Id("c")
				g.Add(dec.IncNonEmpty())
			} else {
				errs.Addf(f.Type.ASTExpr().Pos(), "cannot unmarshal cookie into field of type %s", f.Type)
			}
		})
	}
	g.Line()
}

// BuildErr returns an expression for returning an encore.dev/beta/errs.Error with the given code and message.
func BuildErr(code, msg string) *Statement {
	p := "encore.dev/beta/errs"
	return Qual(p, "B").Call().Dot("Code").Call(Qual(p, code)).Dot("Msg").Call(Lit(msg)).Dot("Err").Call()
}

// BuildErrf is like BuildErr but with a format string.
func BuildErrf(code, format string, args ...Code) *Statement {
	p := "encore.dev/beta/errs"
	args = append([]Code{Lit(format)}, args...)
	return Qual(p, "B").Call().Dot("Code").Call(Qual(p, code)).Dot("Msgf").Call(args...).Dot("Err").Call()
}
