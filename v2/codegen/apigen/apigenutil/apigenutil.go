package apigenutil

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/codegen/internal/genutil"
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
