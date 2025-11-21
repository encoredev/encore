package apigenutil

import (
	"fmt"
	"strings"

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
func DecodeHeaders(g *Group, httpHeaderExpr, paramExpr *Statement, dec *genutil.TypeUnmarshaller, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}

	g.Comment("Decode headers")
	g.Id("h").Op(":=").Add(httpHeaderExpr)
	for _, f := range params {
		singleValExpr := Id("h").Dot("Get").Call(Lit(f.WireName))
		listValExpr := Id("h").Dot("Values").Call(Lit(f.WireName))
		decodeExpr := dec.UnmarshalQueryOrHeader(f.Type, f.WireName, singleValExpr, listValExpr)
		g.Add(paramExpr.Clone().Dot(f.SrcName).Op("=").Add(decodeExpr))
	}
	g.Line()
}

// DecodeQuery is like DecodeHeaders but for query strings.
func DecodeQuery(g *Group, urlValuesExpr, paramExpr *Statement, dec *genutil.TypeUnmarshaller, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}
	g.Comment("Decode query string")
	g.Id("qs").Op(":=").Add(urlValuesExpr)

	for _, f := range params {
		singleValExpr := Id("qs").Dot("Get").Call(Lit(f.WireName))
		listValExpr := Id("qs").Index(Lit(f.WireName))
		decodeExpr := dec.UnmarshalQueryOrHeader(f.Type, f.WireName, singleValExpr, listValExpr)
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

const jsonIterPkg = "github.com/json-iterator/go"

// DecodeBody decodes an io.Reader request body into the given parameters.
func DecodeBody(g *Group, ioReaderExpr *Statement, paramsExpr *Statement, dec *genutil.TypeUnmarshaller, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}

	g.Comment("Decode request body")
	g.Id("payload").Op(":=").Add(dec.ReadBody(ioReaderExpr))
	g.Id("iter").Op(":=").Qual(jsonIterPkg, "ParseBytes").Call(Id("json"), Id("payload"))
	g.Line()

	g.For(Id("iter").Dot("ReadObjectCB").Call(
		Func().Params(Id("_").Op("*").Qual(jsonIterPkg, "Iterator"), Id("key").String()).Bool().Block(
			Switch(Qual("strings", "ToLower").Call(Id("key"))).BlockFunc(func(g *Group) {
				for _, f := range params {
					g.Case(Lit(strings.ToLower(f.WireName))).Block(
						dec.ParseJSON(f.SrcName, Id("iter"), Op("&").Add(paramsExpr.Clone()).Dot(f.SrcName)),
					)
				}
				g.Default().Block(Id("_").Op("=").Id("iter").Dot("SkipAndReturnBytes").Call())
			}),
			Return(True()),
		)).Block(),
	)
	g.Line()
}

// EncodeHeaders generates code for encoding HTTP headers into a http.Header map.
func EncodeHeaders(errs *perr.List, g *Group, httpHeaderExpr, paramExpr *Statement, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}

	g.Line()
	g.Comment("Encode headers")
	g.Add(httpHeaderExpr.Clone().Op("=").Make(Qual("net/http", "Header"), Lit(len(params))))

	for _, f := range params {
		if !schemautil.IsValidHeaderType(f.Type) {
			errs.Addf(f.Type.ASTExpr().Pos(), "cannot marshal %s to header", f.Type)
			continue
		}

		strVals, ok := genutil.MarshalQueryOrHeader(f.Type, paramExpr.Clone().Dot(f.SrcName))
		if !ok {
			errs.Addf(f.Type.ASTExpr().Pos(), "cannot marshal %s to header", f.Type)
			continue
		}

		g.Add(httpHeaderExpr.Clone()).Index(
			Qual("net/textproto", "CanonicalMIMEHeaderKey").Call(Lit(f.WireName)),
		).Op("=").Add(strVals)
	}

	g.Line()
}

// EncodeQuery generates code for encoding URL query values into a url.Values map.
func EncodeQuery(errs *perr.List, g *Group, urlValuesExpr, paramExpr *Statement, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}

	g.Line()
	g.Comment("Encode query string")
	g.Add(urlValuesExpr.Clone().Op("=").Make(Qual("net/url", "Values"), Lit(len(params))))

	for _, f := range params {
		if !schemautil.IsValidQueryType(f.Type) {
			errs.Addf(f.Type.ASTExpr().Pos(), "cannot marshal %s to query string", f.Type)
			continue
		}

		strVals, ok := genutil.MarshalQueryOrHeader(f.Type, paramExpr.Clone().Dot(f.SrcName))
		if !ok {
			errs.Addf(f.Type.ASTExpr().Pos(), "cannot marshal %s to query string", f.Type)
			continue
		}
		g.Add(urlValuesExpr.Clone()).Index(Lit(f.WireName)).Op("=").Add(strVals)
	}

	g.Line()
}

// EncodeBody encodes a request body into the given *jsoniter.Stream.
func EncodeBody(gu *genutil.Helper, g *Group, streamExpr, paramExpr *Statement, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}

	g.Line()
	g.Comment("Encode request body")
	g.Add(streamExpr.Clone().Dot("WriteObjectStart").Call())

	for i, p := range params {
		writeBlock := g

		// If this field is omitted when empty, we need to wrap the write in an if statement.
		if p.OmitEmpty {
			g.If(gu.IsNotJSONEmpty(paramExpr.Clone().Dot(p.SrcName), p.Type)).BlockFunc(func(g *Group) {
				g.Comment(fmt.Sprintf("%s is set to omitempty, so we need to check if it's empty before writing it", p.SrcName))
				writeBlock = g
			})
		}

		writeBlock.Add(streamExpr.Clone().Dot("WriteObjectField").Call(Lit(p.WireName)))
		writeBlock.Add(streamExpr.Clone().Dot("WriteVal").Call(paramExpr.Clone().Dot(p.SrcName)))
		if i+1 < len(params) {
			// If we're not on the last field, write a comma.
			// we do this within the writeBlock so that we don't write a comma if we're omitting the field.
			writeBlock.Add(streamExpr.Clone().Dot("WriteMore").Call())
		}
	}
	g.Add(streamExpr.Clone().Dot("WriteObjectEnd").Call())
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
