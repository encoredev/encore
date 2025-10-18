package clientgen

import (
	"bufio"
	"bytes"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/cockroachdb/errors"

	"encr.dev/internal/version"
	"encr.dev/parser/encoding"
	"encr.dev/pkg/clientgen/clientgentypes"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/idents"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

/* The TypeScript generator generates code that looks like this:
export namespace task {
	export interface AddParams {
		description: string
	}

	export class ServiceClient {
		public Add(params: task_AddParams): Promise<task_AddResponse> {
			// ...
		}
	}
}

*/

// tsGenVersion allows us to introduce breaking changes in the generated code but behind a switch
// meaning that people with client code reliant on the old behaviour can continue to generate the
// old code.
type tsGenVersion int

const (
	// TsInitial is the originally released typescript generator
	TsInitial tsGenVersion = iota

	// TsExperimental can be used to lock experimental or uncompleted features in the generated code
	// It should always be the last item in the enum
	TsExperimental
)

const typescriptGenLatestVersion = TsExperimental - 1

type typescript struct {
	*bytes.Buffer
	md               *meta.Data
	appSlug          string
	typs             *typeRegistry
	currDecl         *schema.Decl
	generatorVersion tsGenVersion
	sharedTypes      bool
	clientTarget     string

	seenJSON           bool // true if a JSON type was seen
	seenStream         bool // true if a stream endpoint was seen
	seenHeaderResponse bool // true if we've seen a header used in a response object
	hasAuth            bool // true if we've seen an authentication handler
	authIsComplexType  bool // true if the auth type is a complex type
}

func (ts *typescript) Version() int {
	return int(ts.generatorVersion)
}

func (ts *typescript) Generate(p clientgentypes.GenerateParams) (err error) {
	defer ts.handleBailout(&err)

	ts.Buffer = p.Buf
	ts.md = p.Meta
	ts.appSlug = p.AppSlug
	ts.typs = getNamedTypes(p.Meta, p.Services)

	if ts.md.AuthHandler != nil {
		if !ts.isAuthCookieOnly() {
			ts.hasAuth = true
			ts.authIsComplexType = ts.md.AuthHandler.Params.GetBuiltin() != schema.Builtin_STRING
		}
	}

	ts.WriteString("// " + doNotEditHeader() + "\n\n")
	ts.WriteString("// Disable eslint, jshint, and jslint for this file.\n")
	ts.WriteString("/* eslint-disable */\n")
	ts.WriteString("/* jshint ignore:start */\n")
	ts.WriteString("/*jslint-disable*/\n")

	if ts.sharedTypes {
		ts.WriteString("import type { CookieWithOptions } from \"encore.dev/api\";\n")
	}

	nss := ts.typs.Namespaces()
	seenNs := make(map[string]bool)
	ts.writeClient(p.Services)
	for _, svc := range p.Meta.Svcs {
		if err := ts.writeService(svc, p.Services, p.Tags); err != nil {
			return err
		}
		seenNs[svc.Name] = true
	}
	if !ts.sharedTypes {
		for _, ns := range nss {
			if !seenNs[ns] {
				ts.writeNamespace(ns)
			}
		}
	}
	ts.writeExtraTypes()
	ts.writeStreamClasses()
	if err := ts.writeBaseClient(p.AppSlug); err != nil {
		return err
	}
	ts.writeCustomErrorType()

	if ts.clientTarget != "" {
		fmt.Fprintf(ts, `
export default new Client(%s, { requestInit: { credentials: "include" } });
`, ts.clientTarget)
	}

	return nil
}

func (ts *typescript) getFields(typ *schema.Type) []*schema.Field {
	if typ == nil {
		return nil
	}
	switch typ.Typ.(type) {
	case *schema.Type_Struct:
		return typ.GetStruct().Fields
	case *schema.Type_Named:
		decl := ts.md.Decls[typ.GetNamed().Id]
		return ts.getFields(decl.Type)
	default:
		return nil
	}
}

func (ts *typescript) isAuthCookieOnly() bool {
	if ts.md.AuthHandler == nil {
		return false
	}
	fields := ts.getFields(ts.md.AuthHandler.Params)
	if fields == nil {
		return false
	}
	for _, field := range fields {
		if field.Wire.GetCookie() == nil {
			return false
		}
	}
	return true
}

func hasPathParams(rpc *meta.RPC) bool {
	return fns.Any(rpc.Path.Segments, func(s *meta.PathSegment) bool {
		return s.Type != meta.PathSegment_LITERAL
	})
}

func (ts *typescript) authImportName() string {
	return fmt.Sprintf("auth_%s", validTSIdentifier(ts.md.AuthHandler.Name))
}

func (ts *typescript) writeAuthType() {
	if ts.sharedTypes {
		fmt.Fprintf(ts, "RequestType<typeof %s>", ts.authImportName())
	} else {
		ts.writeTyp("", ts.md.AuthHandler.Params, 2)
	}
}

func rpcImportName(rpc *meta.RPC) string {
	fileName := strings.TrimSuffix(rpc.Loc.Filename, filepath.Ext(rpc.Loc.Filename))
	return fmt.Sprintf("api_%s_%s_%s", validTSIdentifier(rpc.ServiceName), validTSIdentifier(fileName), validTSIdentifier(rpc.Name))
}

func getMethodType(rpc *meta.RPC) string {
	ts := strings.Builder{}
	for i, method := range rpc.HttpMethods {
		if i > 0 {
			ts.WriteString(" | ")
		}

		if method == "*" {
			ts.WriteString("string")
		} else {
			ts.WriteString("\"" + method + "\"")
		}
	}
	return ts.String()
}

func (ts *typescript) writeService(svc *meta.Service, p clientgentypes.ServiceSet, tags clientgentypes.TagSet) error {
	// Determine if we have anything worth exposing.
	// Either a public RPC or a named type.
	isIncluded := hasPublicRPC(svc) && p.Has(svc.Name)

	decls := ts.typs.Decls(svc.Name)
	if !isIncluded && len(decls) == 0 {
		return nil
	}

	if ts.sharedTypes {
		importsByPath := make(map[string][]string)
		for _, rpc := range svc.Rpcs {
			if rpc.AccessType == meta.RPC_PRIVATE || !tags.IsRPCIncluded(rpc) || rpc.Proto == meta.RPC_RAW ||
				(rpc.ResponseSchema == nil && rpc.RequestSchema == nil && !hasPathParams(rpc)) {
				continue
			}
			path := fmt.Sprintf("~backend/%s/%s", rpc.Loc.PkgName, strings.TrimSuffix(rpc.Loc.Filename, filepath.Ext(rpc.Loc.Filename)))
			importsByPath[path] = append(importsByPath[path], fmt.Sprintf("%s as %s", rpc.Name, rpcImportName(rpc)))
		}
		if len(importsByPath) > 0 {
			ts.WriteString(`/**
 * Import the endpoint handlers to derive the types for the client.
 */
`)
		}
		for _, path := range slices.Sorted(maps.Keys(importsByPath)) {
			imps := fmt.Sprintf(" %s ", importsByPath[path][0])
			if len(importsByPath[path]) > 1 {
				imps = "\n    " + strings.Join(importsByPath[path], ",\n    ") + "\n"
			}
			fmt.Fprintf(ts, "import {%s} from \"%s\";\n", imps, path)
		}
		ts.WriteString("\n")
	}

	ns := svc.Name
	fmt.Fprintf(ts, "export namespace %s {\n", ts.typeName(ns))

	sort.Slice(decls, func(i, j int) bool {
		return decls[i].Name < decls[j].Name
	})
	if !ts.sharedTypes {
		for i, d := range decls {
			if i > 0 {
				ts.WriteString("\n")
			}
			ts.writeDeclDef(ns, d)
		}
	}

	if !isIncluded {
		ts.WriteString("}\n\n")
		return nil
	}
	ts.WriteString("\n")

	numIndent := 1
	indent := func() {
		ts.WriteString(strings.Repeat("    ", numIndent))
	}

	indent()
	fmt.Fprint(ts, "export class ServiceClient {\n")
	numIndent++

	// Constructor
	indent()
	ts.WriteString("private baseClient: BaseClient\n\n")
	indent()
	ts.WriteString("constructor(baseClient: BaseClient) {\n")
	numIndent++
	indent()
	ts.WriteString("this.baseClient = baseClient\n")
	for _, rpc := range svc.Rpcs {
		if rpc.AccessType == meta.RPC_PRIVATE || !tags.IsRPCIncluded(rpc) {
			continue
		}
		name := ts.memberName(rpc.Name)
		indent()
		fmt.Fprintf(ts, "this.%s = this.%s.bind(this)\n", name, name)
	}
	numIndent--
	indent()
	ts.WriteString("}\n")

	// RPCs
	for _, rpc := range svc.Rpcs {
		if rpc.AccessType == meta.RPC_PRIVATE || !tags.IsRPCIncluded(rpc) {
			continue
		}

		ts.WriteByte('\n')

		// Doc string
		if rpc.Doc != nil && *rpc.Doc != "" {
			scanner := bufio.NewScanner(strings.NewReader(*rpc.Doc))
			indent()
			ts.WriteString("/**\n")
			for scanner.Scan() {
				indent()
				ts.WriteString(" * ")
				ts.WriteString(scanner.Text())
				ts.WriteByte('\n')
			}
			indent()
			ts.WriteString(" */\n")
		}

		// Signature
		indent()
		fmt.Fprintf(ts, "public async %s(", ts.memberName(rpc.Name))

		isRaw := rpc.Proto == meta.RPC_RAW

		if isRaw && !ts.sharedTypes {
			fmt.Fprintf(ts, "method: %s, ", getMethodType(rpc))
		}

		nParams := 0
		// Avoid a name collision.
		payloadName := "params"
		segmentPrefix := ""
		if ts.sharedTypes {
			segmentPrefix = payloadName + "."
		}
		var isStream = rpc.StreamingRequest || rpc.StreamingResponse
		var hasHandshake = rpc.HandshakeSchema != nil
		var inlinePathParams = (isRaw || (rpc.RequestSchema == nil && !hasHandshake)) && hasPathParams(rpc) && ts.sharedTypes
		if inlinePathParams {
			ts.WriteString(payloadName + ": { ")
		}
		var rpcPath strings.Builder
		for _, s := range rpc.Path.Segments {
			rpcPath.WriteByte('/')
			if s.Type != meta.PathSegment_LITERAL {
				if !ts.sharedTypes || inlinePathParams {
					if nParams > 0 {
						ts.WriteString(", ")
					}

					ts.WriteString(ts.nonReservedId(s.Value))
					ts.WriteString(": ")
					switch s.ValueType {
					case meta.PathSegment_STRING, meta.PathSegment_UUID:
						ts.WriteString("string")
					case meta.PathSegment_BOOL:
						ts.WriteString("boolean")
					case meta.PathSegment_INT8, meta.PathSegment_INT16, meta.PathSegment_INT32, meta.PathSegment_INT64, meta.PathSegment_INT,
						meta.PathSegment_UINT8, meta.PathSegment_UINT16, meta.PathSegment_UINT32, meta.PathSegment_UINT64, meta.PathSegment_UINT:
						ts.WriteString("number")
					default:
						panic(fmt.Sprintf("unhandled PathSegment type %s", s.ValueType))
					}
					if s.Type == meta.PathSegment_WILDCARD || s.Type == meta.PathSegment_FALLBACK {
						ts.WriteString("[]")
					}
				}
				if s.Type == meta.PathSegment_WILDCARD || s.Type == meta.PathSegment_FALLBACK {
					rpcPath.WriteString("${" + segmentPrefix + ts.nonReservedId(s.Value) + ".map(encodeURIComponent).join(\"/\")}")
				} else {
					rpcPath.WriteString("${encodeURIComponent(" + segmentPrefix + ts.nonReservedId(s.Value) + ")}")
				}
				nParams++
			} else {
				rpcPath.WriteString(s.Value)
			}
		}
		if inlinePathParams {
			ts.WriteString(" }")
		}

		if (!isStream && rpc.RequestSchema != nil) || (isStream && hasHandshake) {
			if !ts.sharedTypes && nParams > 0 {
				ts.WriteString(", ")
			}
			ts.WriteString(payloadName + ": ")
			if ts.sharedTypes {
				fmt.Fprintf(ts, "RequestType<typeof %s>", rpcImportName(rpc))
			} else if isStream {
				ts.writeTyp(ns, rpc.HandshakeSchema, 0)
			} else {
				ts.writeTyp(ns, rpc.RequestSchema, 0)

			}
		} else if isRaw {
			if nParams > 0 {
				ts.WriteString(", ")
			}
			if !ts.sharedTypes {
				ts.WriteString("body?: RequestInit[\"body\"], ")
			}

			if ts.sharedTypes {
				fmt.Fprintf(ts, "options: PickMethods<%s> = {}", getMethodType(rpc))
			} else {
				ts.WriteString("options?: CallParameters")
			}
		}

		var direction streamDirection

		if rpc.StreamingRequest && rpc.StreamingResponse {
			direction = InOut
		} else if rpc.StreamingRequest {
			direction = Out
		} else {
			direction = In
		}

		writeStreamRequest := func(ns string, numIndents int) {
			if rpc.RequestSchema == nil {
				ts.WriteString("void")
			} else if ts.sharedTypes {
				fmt.Fprintf(ts, "StreamRequest<typeof %s>", rpcImportName(rpc))
			} else {
				ts.writeTyp(ns, rpc.RequestSchema, numIndents)
			}
		}
		writeStreamResponse := func(ns string, numIndents int) {
			if rpc.ResponseSchema == nil {
				ts.WriteString("void")
			} else if ts.sharedTypes {
				ts.seenStream = true
				fmt.Fprintf(ts, "StreamResponse<typeof %s>", rpcImportName(rpc))
			} else {
				ts.writeTyp(ns, rpc.ResponseSchema, numIndents)
			}
		}

		ts.WriteString("): Promise<")

		if isStream {
			ts.seenStream = true
			switch direction {
			case InOut:
				ts.WriteString("StreamInOut<")
				writeStreamRequest(ns, 0)
				ts.WriteString(", ")
				writeStreamResponse(ns, 0)
				ts.WriteString(">")
			case In:
				ts.WriteString("StreamIn<")
				writeStreamResponse(ns, 0)
				ts.WriteString(">")
			case Out:
				ts.WriteString("StreamOut<")
				writeStreamRequest(ns, 0)
				ts.WriteString(", ")
				writeStreamResponse(ns, 0)
				ts.WriteString(">")
			}
		} else if rpc.ResponseSchema != nil {
			if ts.sharedTypes {
				fmt.Fprintf(ts, "ResponseType<typeof %s>", rpcImportName(rpc))
			} else {
				ts.writeTyp(ns, rpc.ResponseSchema, 0)
			}
		} else if isRaw {
			ts.WriteString("globalThis.Response")
		} else {
			ts.WriteString("void")
		}
		ts.WriteString("> {\n")

		if isStream {
			if err := ts.streamCallSite(ts.newIdentWriter(numIndent+1), rpc, rpcPath.String(), direction); err != nil {
				return errors.Wrapf(err, "unable to write streaming RPC call site for %ss.%s", rpc.ServiceName, rpc.Name)
			}
		} else {
			if err := ts.rpcCallSite(ns, ts.newIdentWriter(numIndent+1), rpc, rpcPath.String()); err != nil {
				return errors.Wrapf(err, "unable to write RPC call site for %s.%s", rpc.ServiceName, rpc.Name)
			}
		}

		indent()
		ts.WriteString("}\n")
	}
	numIndent--
	indent()
	ts.WriteString("}\n}\n\n")
	return nil
}

func (ts *typescript) streamCallSite(w *indentWriter, rpc *meta.RPC, rpcPath string, direction streamDirection) error {
	headers := ""
	query := ""

	if rpc.HandshakeSchema != nil {
		encs, err := encoding.DescribeRequest(ts.md, rpc.HandshakeSchema, &encoding.Options{SrcNameTag: "json"}, "GET")
		if err != nil {
			return errors.Wrapf(err, "stream %s", rpc.Name)
		}

		handshakeEnc := encs[0]

		if len(handshakeEnc.HeaderParameters) > 0 || len(handshakeEnc.QueryParameters) > 0 {
			w.WriteString("// Convert our params into the objects we need for the request\n")
		}

		// Generate the headers
		if len(handshakeEnc.HeaderParameters) > 0 {
			headers = "headers"

			dict := make(map[string]string)
			for _, field := range handshakeEnc.HeaderParameters {
				ref := ts.Dot("params", field.SrcName)
				dict[field.WireFormat] = ts.convertBuiltinToString(field.Type.GetBuiltin(), ref, field.Optional)
			}

			w.WriteString("const headers = makeRecord<string, string>(")
			ts.Values(w, dict)
			w.WriteString(")\n\n")
		}

		// Generate the query string
		if len(handshakeEnc.QueryParameters) > 0 {
			query = "query"

			dict := make(map[string]string)
			for _, field := range handshakeEnc.QueryParameters {
				if list := field.Type.GetList(); list != nil {
					dot := ts.Dot("params", field.SrcName)
					if field.Optional || ts.isRecursive(field.Type) {
						dot += "?"
					}
					dict[field.WireFormat] = dot +
						".map((v) => " + ts.convertBuiltinToString(list.Elem.GetBuiltin(), "v", field.Optional) + ")"
				} else {
					dict[field.WireFormat] = ts.convertBuiltinToString(
						field.Type.GetBuiltin(),
						ts.Dot("params", field.SrcName),
						field.Optional,
					)
				}
			}

			w.WriteString("const query = makeRecord<string, string | string[]>(")
			ts.Values(w, dict)
			w.WriteString(")\n\n")
		}
	}

	// Build the call to createStream
	var method string

	switch direction {
	case InOut:
		method = "createStreamInOut"
	case In:
		method = "createStreamIn"
	case Out:
		method = "createStreamOut"
	}

	createStream := fmt.Sprintf(
		"this.baseClient.%s(`%s`",
		method,
		rpcPath,
	)

	if headers != "" || query != "" {
		createStream += ", {" + headers

		if headers != "" && query != "" {
			createStream += ", "
		}

		if query != "" {
			createStream += query
		}

		createStream += "}"
	}
	createStream += ")"

	w.WriteStringf("return await %s\n", createStream)
	return nil
}

func (ts *typescript) rpcCallSite(ns string, w *indentWriter, rpc *meta.RPC, rpcPath string) error {
	// Work out how we're going to encode and call this RPC
	rpcEncoding, err := encoding.DescribeRPC(ts.md, rpc, &encoding.Options{SrcNameTag: "json"})
	if err != nil {
		return errors.Wrapf(err, "rpc %s", rpc.Name)
	}

	// Raw end points just pass through the request
	// and need no further code generation
	if rpc.Proto == meta.RPC_RAW {
		if ts.sharedTypes {
			w.WriteStringf("options.method ||= \"%s\";\n", rpcEncoding.DefaultMethod)
		}
		w.WriteString("return this.baseClient.callAPI(")
		if ts.sharedTypes {
			w.WriteStringf(
				"`%s`, options",
				rpcPath,
			)

		} else {
			w.WriteStringf(
				"method, `%s`, body, options",
				rpcPath,
			)
		}
		w.WriteString(")\n")
		return nil
	}

	// Work out how we encode the Request Schema
	headers := ""
	query := ""
	body := ""

	if rpc.RequestSchema != nil {
		reqEnc := rpcEncoding.DefaultRequestEncoding

		if len(reqEnc.HeaderParameters) > 0 || len(reqEnc.QueryParameters) > 0 {
			w.WriteString("// Convert our params into the objects we need for the request\n")
		}

		// Generate the headers
		if len(reqEnc.HeaderParameters) > 0 {
			headers = "headers"

			dict := make(map[string]string)
			for _, field := range reqEnc.HeaderParameters {
				ref := ts.Dot("params", field.SrcName)
				dict[field.WireFormat] = ts.convertBuiltinToString(field.Type.GetBuiltin(), ref, field.Optional)
			}

			w.WriteString("const headers = makeRecord<string, string>(")
			ts.Values(w, dict)
			w.WriteString(")\n\n")
		}

		// Generate the query string
		if len(reqEnc.QueryParameters) > 0 {
			query = "query"

			dict := make(map[string]string)
			for _, field := range reqEnc.QueryParameters {
				if list := field.Type.GetList(); list != nil {
					dot := ts.Dot("params", field.SrcName)
					if field.Optional || ts.isRecursive(field.Type) {
						dot += "?"
					}
					dict[field.WireFormat] = dot +
						".map((v) => " + ts.convertBuiltinToString(list.Elem.GetBuiltin(), "v", field.Optional) + ")"
				} else {
					dict[field.WireFormat] = ts.convertBuiltinToString(
						field.Type.GetBuiltin(),
						ts.Dot("params", field.SrcName),
						field.Optional,
					)
				}
			}

			w.WriteString("const query = makeRecord<string, string | string[]>(")
			ts.Values(w, dict)
			w.WriteString(")\n\n")
		}

		// Generate the body
		if len(reqEnc.BodyParameters) > 0 {
			if len(reqEnc.HeaderParameters) == 0 && len(reqEnc.QueryParameters) == 0 && (!ts.sharedTypes || !hasPathParams(rpc)) {
				// In the simple case we can just encode the params as the body directly
				body = "JSON.stringify(params)"
			} else {
				// Else we need a new struct called "body"
				body = "JSON.stringify(body)"

				dict := make(map[string]string)
				for _, field := range reqEnc.BodyParameters {
					dict[field.WireFormat] = ts.Dot("params", field.SrcName)
				}

				w.WriteString("// Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)\nconst body: Record<string, any> = ")
				ts.Values(w, dict)
				w.WriteString("\n\n")
			}
		}
	}

	// Build the call to callTypedAPI
	callAPI := "this.baseClient.callTypedAPI("
	if !ts.sharedTypes {
		callAPI += fmt.Sprintf("\"%s\", ", rpcEncoding.DefaultMethod)
	}
	callAPI += fmt.Sprintf("`%s`", rpcPath)
	if body != "" || headers != "" || query != "" || ts.sharedTypes {
		if body == "" {
			body = "undefined"
		}
		if !ts.sharedTypes {
			callAPI += ", " + body
		}

		if headers != "" || query != "" || ts.sharedTypes {
			callAPI += ", {" + headers

			if headers != "" && query != "" {
				callAPI += ", "
			}

			if query != "" {
				callAPI += query
			}

			if ts.sharedTypes {
				if headers != "" || query != "" {
					callAPI += ", "
				}
				callAPI += fmt.Sprintf(`method: "%s", body: %s`, rpcEncoding.DefaultMethod, body)
			}
			callAPI += "}"
		}
	}
	callAPI += ")"

	// If there's no response schema, we can just return the call to the API directly
	if rpc.ResponseSchema == nil {
		w.WriteStringf("await %s\n", callAPI)
		return nil
	}

	w.WriteStringf("// Now make the actual call to the API\nconst resp = await %s\n", callAPI)

	respEnc := rpcEncoding.ResponseEncoding

	// If we don't need to do anything with the body, we can just return the response
	if len(respEnc.HeaderParameters) == 0 {
		if ts.sharedTypes {
			w.WriteString("return JSON.parse(await resp.text(), dateReviver) as ")
			fmt.Fprintf(ts, "ResponseType<typeof %s>", rpcImportName(rpc))
		} else {
			w.WriteString("return await resp.json() as ")
			ts.writeTyp(ns, rpc.ResponseSchema, 0)
		}
		w.WriteString("\n")
		return nil
	}

	// Otherwise, we need to add the header fields to the response
	w.WriteString("\n//Populate the return object from the JSON body and received headers\n")

	if ts.sharedTypes {
		w.WriteStringf("const rtn = JSON.parse(await resp.text(), dateReviver) as ResponseType<typeof %s>", rpcImportName(rpc))
	} else {
		w.WriteString("const rtn = await resp.json() as ")
		ts.writeTyp(ns, rpc.ResponseSchema, 0)
	}
	w.WriteString("\n")

	for _, headerField := range respEnc.HeaderParameters {
		isSetCookie := strings.ToLower(headerField.WireFormat) == "set-cookie"
		isSetCookieArray := isSetCookie && headerField.Type.GetList() != nil

		if isSetCookie {
			w.WriteString("// Skip set-cookie header in browser context as browsers doesn't have access to read it\n")
			w.WriteString("if (!BROWSER) {\n")
			w = w.Indent()
		}

		ts.seenHeaderResponse = true

		if isSetCookieArray {
			// Handle multiple Set-Cookie headers
			fieldValue := fmt.Sprintf("resp.headers.getAll(\"%s\")", headerField.WireFormat)
			w.WriteStringf("%s = %s\n", ts.Dot("rtn", headerField.SrcName), fieldValue)
		} else {
			// Handle single value headers (including single Set-Cookie)
			fieldValue := fmt.Sprintf("mustBeSet(\"Header `%s`\", resp.headers.get(\"%s\"))", headerField.WireFormat, headerField.WireFormat)
			w.WriteStringf("%s = %s\n", ts.Dot("rtn", headerField.SrcName), ts.convertStringToBuiltin(headerField.Type.GetBuiltin(), fieldValue))
		}

		if isSetCookie {
			w = w.Dedent()
			w.WriteString("}\n")
		}
	}

	w.WriteString("return rtn\n")
	return nil
}

// nonReservedId returns the given ID, unless we have it a reserved within the client function _or_ it's a reserved Typescript keyword
func (ts *typescript) nonReservedId(id string) string {
	switch id {
	// our reserved keywords (or ID's we use within the generated client functions)
	case "params", "headers", "query", "body", "resp", "rtn":
		return "_" + id

	// Typescript & Javascript keywords
	case "abstract", "any", "arguments", "as", "async", "await", "boolean", "break", "byte", "case", "catch", "char",
		"class", "const", "constructor", "continue", "debugger", "declare", "default", "delete", "do", "double", "else",
		"enum", "eval", "export", "extends", "false", "final", "finally", "float", "for", "from", "function", "get",
		"goto", "if", "implements", "import", "in", "instanceof", "interface", "let", "long", "module", "namespace",
		"native", "new", "null", "number", "of", "package", "private", "protected", "public", "require", "return",
		"set", "short", "static", "string", "super", "switch", "symbol", "synchronized", "this", "throw", "throws",
		"transient", "true", "try", "type", "typeof", "var", "void", "volatile", "while", "with", "yield":
		return "_" + id

	default:
		return id
	}
}

func (ts *typescript) writeNamespace(ns string) {
	decls := ts.typs.Decls(ns)
	if len(decls) == 0 {
		return
	}

	fmt.Fprintf(ts, "export namespace %s {\n", ts.typeName(ns))
	sort.Slice(decls, func(i, j int) bool {
		return decls[i].Name < decls[j].Name
	})
	for i, d := range decls {
		if i > 0 {
			ts.WriteString("\n")
		}

		ts.writeDeclDef(ns, d)
	}
	ts.WriteString("}\n\n")
}

func (ts *typescript) writeDeclDef(ns string, decl *schema.Decl) {
	if decl.Doc != "" {
		scanner := bufio.NewScanner(strings.NewReader(decl.Doc))
		ts.WriteString("    /**\n")
		for scanner.Scan() {
			ts.WriteString("     * ")
			ts.WriteString(scanner.Text())
			ts.WriteByte('\n')
		}
		ts.WriteString("     */\n")
	}

	var typeParams strings.Builder
	if len(decl.TypeParams) > 0 {
		typeParams.WriteRune('<')

		for i, typeParam := range decl.TypeParams {
			if i > 0 {
				typeParams.WriteString(", ")
			}

			typeParams.WriteString(typeParam.Name)
		}

		typeParams.WriteRune('>')
	}

	// If it's a struct type, expose it as an interface;
	// other types should be type aliases.
	if st := decl.Type.GetStruct(); st != nil {
		fmt.Fprintf(ts, "    export interface %s%s ", ts.typeName(decl.Name), typeParams.String())
	} else {
		fmt.Fprintf(ts, "    export type %s%s = ", ts.typeName(decl.Name), typeParams.String())
	}
	ts.currDecl = decl
	ts.writeTyp(ns, decl.Type, 1)
	ts.WriteString("\n")
}

func (ts *typescript) writeStreamClasses() {
	if ts.sharedTypes {
		ts.WriteString(`
import {
  StreamInOutHandlerFn,
  StreamInHandlerFn,
  StreamOutHandlerFn,
} from "encore.dev/api";

type StreamRequest<Type> = Type extends
  | StreamInOutHandlerFn<any, infer Req, any>
  | StreamInHandlerFn<any, infer Req, any>
  | StreamOutHandlerFn<any, any>
  ? Req
  : never;

type StreamResponse<Type> = Type extends
  | StreamInOutHandlerFn<any, any, infer Resp>
  | StreamInHandlerFn<any, any, infer Resp>
  | StreamOutHandlerFn<any, infer Resp>
  ? Resp
  : never;

`)
	}

	parse := "JSON.parse(event.data)"
	if ts.sharedTypes {
		parse = "JSON.parse(event.data, dateReviver)"
	}

	send := `
    async send(msg: Request) {
        if (this.socket.ws.readyState === WebSocket.CONNECTING) {
            // await that the socket is opened
            await new Promise((resolve) => {
                this.socket.ws.addEventListener("open", resolve, { once: true });
            });
        }

        return this.socket.ws.send(JSON.stringify(msg));
    }`

	receive := `
    async next(): Promise<Response | undefined> {
        for await (const next of this) return next;
        return undefined;
    }

    async *[Symbol.asyncIterator](): AsyncGenerator<Response, undefined, void> {
        while (true) {
            if (this.buffer.length > 0) {
                yield this.buffer.shift() as Response;
            } else {
                if (this.socket.ws.readyState === WebSocket.CLOSED) return;
                await this.socket.hasUpdate();
            }
        }
    }`

	ts.WriteString(`
function encodeWebSocketHeaders(headers: Record<string, string>) {
    // url safe, no pad
    const base64encoded = btoa(JSON.stringify(headers))
      .replaceAll("=", "")
      .replaceAll("+", "-")
      .replaceAll("/", "_");
    return "encore.dev.headers." + base64encoded;
}

class WebSocketConnection {
    public ws: WebSocket;

    private hasUpdateHandlers: (() => void)[] = [];

    constructor(url: string, headers?: Record<string, string>) {
        let protocols = ["encore-ws"];
        if (headers) {
            protocols.push(encodeWebSocketHeaders(headers))
        }

        this.ws = new WebSocket(url, protocols)

        this.on("error", () => {
            this.resolveHasUpdateHandlers();
        });

        this.on("close", () => {
            this.resolveHasUpdateHandlers();
        });
    }

    resolveHasUpdateHandlers() {
        const handlers = this.hasUpdateHandlers;
        this.hasUpdateHandlers = [];

        for (const handler of handlers) {
            handler()
        }
    }

    async hasUpdate() {
        // await until a new message have been received, or the socket is closed
        await new Promise((resolve) => {
            this.hasUpdateHandlers.push(() => resolve(null))
        });
    }

    on(type: "error" | "close" | "message" | "open", handler: (event: any) => void) {
        this.ws.addEventListener(type, handler);
    }

    off(type: "error" | "close" | "message" | "open", handler: (event: any) => void) {
        this.ws.removeEventListener(type, handler);
    }

    close() {
        this.ws.close();
    }
}

export class StreamInOut<Request, Response> {
    public socket: WebSocketConnection;
    private buffer: Response[] = [];

    constructor(url: string, headers?: Record<string, string>) {
        this.socket = new WebSocketConnection(url, headers);
        this.socket.on("message", (event: any) => {
            this.buffer.push(` + parse + `);
            this.socket.resolveHasUpdateHandlers();
        });
    }

    close() {
        this.socket.close();
    }
` + send + `
` + receive + `
}

export class StreamIn<Response> {
    public socket: WebSocketConnection;
    private buffer: Response[] = [];

    constructor(url: string, headers?: Record<string, string>) {
        this.socket = new WebSocketConnection(url, headers);
        this.socket.on("message", (event: any) => {
            this.buffer.push(` + parse + `);
            this.socket.resolveHasUpdateHandlers();
        });
    }

    close() {
        this.socket.close();
    }
` + receive + `
}

export class StreamOut<Request, Response> {
    public socket: WebSocketConnection;
    private responseValue: Promise<Response>;

    constructor(url: string, headers?: Record<string, string>) {
        let responseResolver: (_: any) => void;
        this.responseValue = new Promise((resolve) => responseResolver = resolve);

        this.socket = new WebSocketConnection(url, headers);
        this.socket.on("message", (event: any) => {
            responseResolver(` + parse + `)
        });
    }

    async response(): Promise<Response> {
        return this.responseValue;
    }

    close() {
        this.socket.close();
    }
` + send + `
}`)
}

func (ts *typescript) writeClient(set clientgentypes.ServiceSet) {
	w := ts.newIdentWriter(0)
	w.WriteStringf(`
/**
 * BaseURL is the base URL for calling the Encore application's API.
 */
export type BaseURL = string

export const Local: BaseURL = "http://localhost:4000"

/**
 * Environment returns a BaseURL for calling the cloud environment with the given name.
 */
export function Environment(name: string): BaseURL {
    return `+"`https://${name}-"+ts.appSlug+".encr.app`"+`
}

/**
 * PreviewEnv returns a BaseURL for calling the preview environment with the given PR number.
 */
export function PreviewEnv(pr: number | string): BaseURL {
    return Environment(`+"`pr${pr}`"+`)
}

const BROWSER = typeof globalThis === "object" && ("window" in globalThis);

/**
 * Client is an API client for the `+ts.appSlug+` Encore application.
 */
export %sclass Client {
`, func() string {
		if ts.clientTarget != "" {
			return ""
		}
		return "default "
	}())

	{
		w := w.Indent()

		for _, svc := range ts.md.Svcs {
			if hasPublicRPC(svc) && set.Has(svc.Name) {
				w.WriteStringf("public readonly %s: %s.ServiceClient\n", ts.memberName(svc.Name), ts.typeName(svc.Name))
			}
		}
		w.WriteString("private readonly options: ClientOptions\n")
		w.WriteString("private readonly target: string\n")
		w.WriteString("\n")

		// Only include the deprecated constructor if bearer token authentication is being used
		if ts.hasAuth && !ts.authIsComplexType {
			w.WriteString(`
/**
 * @deprecated This constructor is deprecated, and you should move to using BaseURL with an Options object
 */
constructor(target: string, token?: string)
`)
		}

		w.WriteString(`
/**
 * Creates a Client for calling the public and authenticated APIs of your Encore application.
 *
 * @param target  The target which the client should be configured to use. See Local and Environment for options.
 * @param options Options for the client
 */
constructor(target: BaseURL, options?: ClientOptions)`)

		if ts.hasAuth && !ts.authIsComplexType {
			w.WriteString("\nconstructor(target: string | BaseURL = \"prod\", options?: string | ClientOptions) {\n")
			{
				w := w.Indent()

				w.WriteString(`
// Convert the old constructor parameters to a BaseURL object and a ClientOptions object
if (!target.startsWith("http://") && !target.startsWith("https://")) {
    target = Environment(target)
}

if (typeof options === "string") {
    options = { auth: options }
}

`)
			}
		} else {
			w.WriteString(" {\n")
		}

		{
			w := w.Indent()

			w.WriteString("this.target = target\n")
			w.WriteString("this.options = options ?? {}\n")
			w.WriteString("const base = new BaseClient(this.target, this.options)\n")
			for _, svc := range ts.md.Svcs {
				if hasPublicRPC(svc) && set.Has(svc.Name) {
					w.WriteStringf("this.%s = new %s.ServiceClient(base)\n", ts.memberName(svc.Name), ts.typeName(svc.Name))
				}
			}
		}
		w.WriteString("}\n")
	}

	w.WriteString(`
    /**
     * Creates a new Encore client with the given client options set.
     *
     * @param options Client options to set. They are merged with existing options.
     **/
    public with(options: ClientOptions): Client {
        return new Client(this.target, {
            ...this.options,
            ...options,
        })
    }
`)

	w.WriteString("}\n")

	handler := ts.md.AuthHandler
	if ts.hasAuth && ts.sharedTypes && ts.authIsComplexType {

		ts.WriteString(`
/**
 * Import the auth handler to be able to derive the auth type
 */
`)
		fmt.Fprintf(ts, `import type { %s as %s } from "~backend/%s/%s";`,
			handler.Name,
			ts.authImportName(),
			handler.Loc.PkgName,
			strings.TrimSuffix(handler.Loc.Filename, filepath.Ext(handler.Loc.Filename)))
		ts.WriteString("\n")
	}

	w.WriteString(`
/**
 * ClientOptions allows you to override any default behaviour within the generated Encore client.
 */
export interface ClientOptions {
    /**
     * By default the client will use the inbuilt fetch function for making the API requests.
     * however you can override it with your own implementation here if you want to run custom
     * code on each API request made or response received.
     */
    fetcher?: Fetcher

    /** Default RequestInit to be used for the client */
    requestInit?: Omit<RequestInit, "headers"> & { headers?: Record<string, string> }
`)

	if ts.hasAuth {
		if !ts.authIsComplexType {
			w.WriteString(`
    /**
     * Allows you to set the auth token to be used for each request
     * either by passing in a static token string or by passing in a function
     * which returns the auth token.
     *
     * These tokens will be sent as bearer tokens in the Authorization header.
     */
`)
		} else {
			w.WriteString(`
    /**
     * Allows you to set the authentication data to be used for each
     * request either by passing in a static object or by passing in
     * a function which returns a new object for each request.
     */
`)
		}

		w.WriteString("    auth?: ")
		ts.writeAuthType()
		w.WriteString(" | AuthDataGenerator\n")
	}

	w.WriteString(`}

`)
}

func (ts *typescript) writeBaseClient(appSlug string) error {
	userAgent := fmt.Sprintf("%s-Generated-TS-Client (Encore/%s)", appSlug, version.Version)

	reqOmit := `"method" | "body" | "headers"`
	if ts.sharedTypes {
		reqOmit = `"headers"`
	}
	fmt.Fprintf(ts, `
// CallParameters is the type of the parameters to a method call, but require headers to be a Record type
type CallParameters = Omit<RequestInit, %s> & {
    /** Headers to be sent with the request */
    headers?: Record<string, string>

    /** Query parameters to be sent with the request */
    query?: Record<string, string | string[]>
}
`, reqOmit)

	if ts.hasAuth {
		ts.WriteString(`
// AuthDataGenerator is a function that returns a new instance of the authentication data required by this API
export type AuthDataGenerator = () =>
  | `)
		ts.writeAuthType()
		ts.WriteString(`
  | Promise<`)
		ts.writeAuthType()
		ts.WriteString(` | undefined>
  | undefined;`)
	}

	ts.WriteString(`

// A fetcher is the prototype for the inbuilt Fetch function
export type Fetcher = typeof fetch;

const boundFetch = fetch.bind(this);

class BaseClient {
    readonly baseURL: string
    readonly fetcher: Fetcher
    readonly headers: Record<string, string>
    readonly requestInit: Omit<RequestInit, "headers"> & { headers?: Record<string, string> }`)

	if ts.hasAuth {
		ts.WriteString("\n    readonly authGenerator?: AuthDataGenerator")
	}

	ts.WriteString(`

    constructor(baseURL: string, options: ClientOptions) {
        this.baseURL = baseURL
        this.headers = {}

        // Add User-Agent header if the script is running in the server
        // because browsers do not allow setting User-Agent headers to requests
        if (!BROWSER) {
            this.headers["User-Agent"] = "` + userAgent + `";
        }

        this.requestInit = options.requestInit ?? {};

        // Setup what fetch function we'll be using in the base client
        if (options.fetcher !== undefined) {
            this.fetcher = options.fetcher
        } else {
            this.fetcher = boundFetch
        }`)

	if ts.hasAuth {
		ts.WriteString(`

        // Setup an authentication data generator using the auth data token option
        if (options.auth !== undefined) {
            const auth = options.auth
            if (typeof auth === "function") {
                this.authGenerator = auth
            } else {
                this.authGenerator = () => auth
            }
        }`)
	}

	ts.WriteString(`
    }

    async getAuthData(): Promise<CallParameters | undefined> {`)
	if ts.hasAuth {
		ts.WriteString(`
        let authData: `)
		ts.writeAuthType()
		ts.WriteString(` | undefined;

        // If authorization data generator is present, call it and add the returned data to the request
        if (this.authGenerator) {
            const mayBePromise = this.authGenerator();
            if (mayBePromise instanceof Promise) {
                authData = await mayBePromise;
            } else {
                authData = mayBePromise;
            }
        }

        if (authData) {
            const data: CallParameters = {};

`)

		w := ts.newIdentWriter(3)
		if ts.authIsComplexType {
			authData, err := encoding.DescribeAuth(ts.md, ts.md.AuthHandler.Params, &encoding.Options{SrcNameTag: "json"})
			if err != nil {
				return errors.Wrap(err, "unable to describe auth data")
			}

			// Generate the query string
			if len(authData.QueryParameters) > 0 {
				dict := make(map[string]string)
				for _, field := range authData.QueryParameters {
					if list := field.Type.GetList(); list != nil {
						dot := ts.Dot("authData", field.SrcName)
						if field.Optional || ts.isRecursive(field.Type) {
							dot += "?"
						}
						dict[field.WireFormat] = dot +
							".map((v) => " + ts.convertBuiltinToString(list.Elem.GetBuiltin(), "v", field.Optional) + ")"
					} else {
						dict[field.WireFormat] = ts.convertBuiltinToString(
							field.Type.GetBuiltin(),
							ts.Dot("authData", field.SrcName),
							field.Optional,
						)
					}
				}

				w.WriteString("data.query = makeRecord<string, string | string[]>(")
				ts.Values(w, dict)
				w.WriteString(");\n")
			}

			// Generate the headers
			if len(authData.HeaderParameters) > 0 {
				dict := make(map[string]string)
				for _, field := range authData.HeaderParameters {
					ref := ts.Dot("authData", field.SrcName)
					dict[field.WireFormat] = ts.convertBuiltinToString(field.Type.GetBuiltin(), ref, field.Optional)
				}

				w.WriteString("data.headers = makeRecord<string, string>(")
				ts.Values(w, dict)
				w.WriteString(");\n")
			}
		} else {
			w.WriteString("data.headers = {};\n")
			w.WriteString("data.headers[\"Authorization\"] = \"Bearer \" + authData;\n")
		}

		w.WriteString("\nreturn data;\n")
		w.Dedent().WriteString("}\n")
	}

	ts.WriteString(`
        return undefined;
    }
`)

	ts.WriteString(`
    // createStreamInOut sets up a stream to a streaming API endpoint.
    async createStreamInOut<Request, Response>(path: string, params?: CallParameters): Promise<StreamInOut<Request, Response>> {
        let { query, headers } = params ?? {};

        // Fetch auth data if there is any
        const authData = await this.getAuthData();

        // If we now have authentication data, add it to the request
        if (authData) {
            if (authData.query) {
                query = {...query, ...authData.query};
            }
            if (authData.headers) {
                headers = {...headers, ...authData.headers};
            }
        }

        const queryString = query ? '?' + encodeQuery(query) : ''
        return new StreamInOut(this.baseURL + path + queryString, headers);
    }

    // createStreamIn sets up a stream to a streaming API endpoint.
    async createStreamIn<Response>(path: string, params?: CallParameters): Promise<StreamIn<Response>> {
        let { query, headers } = params ?? {};

        // Fetch auth data if there is any
        const authData = await this.getAuthData();

        // If we now have authentication data, add it to the request
        if (authData) {
            if (authData.query) {
                query = {...query, ...authData.query};
            }
            if (authData.headers) {
                headers = {...headers, ...authData.headers};
            }
        }

        const queryString = query ? '?' + encodeQuery(query) : ''
        return new StreamIn(this.baseURL + path + queryString, headers);
    }

    // createStreamOut sets up a stream to a streaming API endpoint.
    async createStreamOut<Request, Response>(path: string, params?: CallParameters): Promise<StreamOut<Request, Response>> {
        let { query, headers } = params ?? {};

        // Fetch auth data if there is any
        const authData = await this.getAuthData();

        // If we now have authentication data, add it to the request
        if (authData) {
            if (authData.query) {
                query = {...query, ...authData.query};
            }
            if (authData.headers) {
                headers = {...headers, ...authData.headers};
            }
        }

        const queryString = query ? '?' + encodeQuery(query) : ''
        return new StreamOut(this.baseURL + path + queryString, headers);
    }
`)

	callParams := "method: string, path: string, body?: RequestInit[\"body\"], params?: CallParameters"
	callAPIParams := "method, path, body"
	initParams := `
            method,
            body: body ?? null,`
	if ts.sharedTypes {
		callParams = "path: string, params?: CallParameters"
		callAPIParams = "path"
		initParams = ""
	}

	fmt.Fprintf(ts, `
    // callTypedAPI makes an API call, defaulting content type to "application/json"
    public async callTypedAPI(%s): Promise<Response> {
        return this.callAPI(%s, {
            ...params,
            headers: { "Content-Type": "application/json", ...params?.headers }
        });
    }
`, callParams, callAPIParams)

	fmt.Fprintf(ts, `
    // callAPI is used by each generated API method to actually make the request
    public async callAPI(%s): Promise<Response> {
        let { query, headers, ...rest } = params ?? {}
        const init = {
            ...this.requestInit,
            ...rest,%s
        }
`, callParams, initParams)

	ts.WriteString(`
        // Merge our headers with any predefined headers
        init.headers = {...this.headers, ...init.headers, ...headers}

        // Fetch auth data if there is any
        const authData = await this.getAuthData();

        // If we now have authentication data, add it to the request
        if (authData) {
            if (authData.query) {
                query = {...query, ...authData.query};
            }
            if (authData.headers) {
                init.headers = {...init.headers, ...authData.headers};
            }
        }

        // Make the actual request
        const queryString = query ? '?' + encodeQuery(query) : ''
        const response = await this.fetcher(this.baseURL+path+queryString, init)

        // handle any error responses
        if (!response.ok) {
            // try and get the error message from the response body
            let body: APIErrorResponse = { code: ErrCode.Unknown, message: ` + "`request failed: status ${response.status}`" + ` }

            // if we can get the structured error we should, otherwise give a best effort
            try {
                const text = await response.text()

                try {
                    const jsonBody = JSON.parse(text)
                    if (isAPIErrorResponse(jsonBody)) {
                        body = jsonBody
                    } else {
                        body.message += ": " + JSON.stringify(jsonBody)
                    }
                } catch {
                    body.message += ": " + text
                }
            } catch (e) {
                // otherwise we just append the text to the error message
                body.message += ": " + String(e)
            }

            throw new APIError(response.status, body)
        }

        return response
    }
}`)
	return nil
}

func (ts *typescript) writeExtraTypes() {
	if ts.seenJSON {
		ts.WriteString(`// JSONValue represents an arbitrary JSON value.
export type JSONValue = string | number | boolean | null | JSONValue[] | {[key: string]: JSONValue}
`)
	}
	if ts.sharedTypes {
		ts.WriteString(`
type PickMethods<Type> = Omit<CallParameters, "method"> & { method?: Type };

// Helper type to omit all fields that are cookies.
type OmitCookie<T> = {
  [K in keyof T as T[K] extends CookieWithOptions<any> ? never : K]: T[K];
};

type RequestType<Type extends (...args: any[]) => any> =
  Parameters<Type> extends [infer H, ...any[]]
    ? OmitCookie<H>
    : void;

type ResponseType<Type extends (...args: any[]) => any> = OmitCookie<Awaited<ReturnType<Type>>>;

function dateReviver(key: string, value: any): any {
  if (
    typeof value === "string" &&
    value.length >= 10 &&
    value.charCodeAt(0) >= 48 && // '0'
    value.charCodeAt(0) <= 57 // '9'
  ) {
    const parsedDate = new Date(value);
    if (!isNaN(parsedDate.getTime())) {
      return parsedDate;
    }
  }
  return value;
}
`)
	}

	ts.WriteString(`

function encodeQuery(parts: Record<string, string | string[]>): string {
    const pairs: string[] = []
    for (const key in parts) {
        const val = (Array.isArray(parts[key]) ?  parts[key] : [parts[key]]) as string[]
        for (const v of val) {
            pairs.push(` + "`" + `${key}=${encodeURIComponent(v)}` + "`" + `)
        }
    }
    return pairs.join("&")
}

// makeRecord takes a record and strips any undefined values from it,
// and returns the same record with a narrower type.
// @ts-ignore - TS ignore because makeRecord is not always used
function makeRecord<K extends string | number | symbol, V>(record: Record<K, V | undefined>): Record<K, V> {
    for (const key in record) {
        if (record[key] === undefined) {
            delete record[key]
        }
    }
    return record as Record<K, V>
}
`)

	if ts.seenHeaderResponse {
		ts.WriteString(`

// mustBeSet will throw an APIError with the Data Loss code if value is null or undefined
function mustBeSet<A>(field: string, value: A | null | undefined): A {
    if (value === null || value === undefined) {
        throw new APIError(
            500,
            {
                code: ErrCode.DataLoss,
                message: ` + "`${field} was unexpectedly ${value}`" + `, // ${value} will create the string "null" or "undefined"
            },
        )
    }
    return value
}
`)
	}
}

func (ts *typescript) writeDecl(ns string, decl *schema.Decl) {
	if decl.Loc.PkgName != ns {
		ts.WriteString(ts.typeName(decl.Loc.PkgName) + ".")
	}
	ts.WriteString(ts.typeName(decl.Name))
}

func (ts *typescript) builtinType(typ schema.Builtin) string {
	switch typ {
	case schema.Builtin_ANY:
		return "any"
	case schema.Builtin_BOOL:
		return "boolean"
	case schema.Builtin_INT, schema.Builtin_INT8, schema.Builtin_INT16, schema.Builtin_INT32, schema.Builtin_INT64,
		schema.Builtin_UINT, schema.Builtin_UINT8, schema.Builtin_UINT16, schema.Builtin_UINT32, schema.Builtin_UINT64,
		schema.Builtin_FLOAT32, schema.Builtin_FLOAT64:
		return "number"
	case schema.Builtin_STRING:
		return "string"
	case schema.Builtin_BYTES:
		return "string" // TODO
	case schema.Builtin_TIME:
		return "string" // TODO
	case schema.Builtin_JSON:
		ts.seenJSON = true
		return "JSONValue"
	case schema.Builtin_UUID:
		return "string"
	case schema.Builtin_USER_ID:
		return "string"
	case schema.Builtin_DECIMAL:
		return "string"
	default:
		ts.errorf("unknown builtin type %v", typ)
		return "any"
	}
}

func (ts *typescript) convertBuiltinToString(typ schema.Builtin, val string, isOptional bool) string {
	var code string
	switch typ {
	case schema.Builtin_STRING:
		return val
	case schema.Builtin_JSON:
		code = fmt.Sprintf("JSON.stringify(%s)", val)
	case schema.Builtin_TIME:
		if ts.sharedTypes {
			// If we're using shared types then this will actually be a Date object.
			// Otherwise it will be a string.
			code = fmt.Sprintf("%s.toISOString()", val)
		} else {
			code = fmt.Sprintf("String(%s)", val)
		}
	default:
		code = fmt.Sprintf("String(%s)", val)
	}

	if isOptional {
		code = fmt.Sprintf("%s === undefined ? undefined : %s", val, code)
	}
	return code
}

func (ts *typescript) convertStringToBuiltin(typ schema.Builtin, val string) string {
	switch typ {
	case schema.Builtin_ANY:
		return val
	case schema.Builtin_BOOL:
		return fmt.Sprintf("%s.toLowerCase() === \"true\"", val)
	case schema.Builtin_INT, schema.Builtin_INT8, schema.Builtin_INT16, schema.Builtin_INT32, schema.Builtin_INT64,
		schema.Builtin_UINT, schema.Builtin_UINT8, schema.Builtin_UINT16, schema.Builtin_UINT32, schema.Builtin_UINT64:
		return fmt.Sprintf("parseInt(%s, 10)", val)
	case schema.Builtin_FLOAT32, schema.Builtin_FLOAT64:
		return fmt.Sprintf("Number(%s)", val)
	case schema.Builtin_STRING:
		return val
	case schema.Builtin_BYTES:
		return val
	case schema.Builtin_TIME:
		return val
	case schema.Builtin_JSON:
		ts.seenJSON = true
		return fmt.Sprintf("JSON.parse(%s)", val)
	case schema.Builtin_UUID:
		return val
	case schema.Builtin_USER_ID:
		return val
	default:
		ts.errorf("unknown builtin type %v", typ)
		return "any"
	}
}

func (ts *typescript) writeTyp(ns string, typ *schema.Type, numIndents int) {
	switch typ := typ.Typ.(type) {
	case *schema.Type_Named:
		decl := ts.md.Decls[typ.Named.Id]
		ts.writeDecl(ns, decl)

		// Write the type arguments
		if len(typ.Named.TypeArguments) > 0 {
			ts.WriteRune('<')

			for i, typeArg := range typ.Named.TypeArguments {
				if i > 0 {
					ts.WriteString(", ")
				}

				ts.writeTyp(ns, typeArg, 0)
			}

			ts.WriteRune('>')
		}
	case *schema.Type_List:
		elem := typ.List.Elem
		union, isUnion := elem.Typ.(*schema.Type_Union)
		paren := isUnion && len(union.Union.Types) > 1

		if paren {
			ts.WriteString("(")
		}
		ts.writeTyp(ns, elem, numIndents)
		if paren {
			ts.WriteString(")")
		}

		ts.WriteString("[]")

	case *schema.Type_Map:
		ts.WriteString("{ [key: ")
		ts.writeTyp(ns, typ.Map.Key, numIndents)
		ts.WriteString("]: ")
		ts.writeTyp(ns, typ.Map.Value, numIndents)
		ts.WriteString(" }")

	case *schema.Type_Builtin:
		ts.WriteString(ts.builtinType(typ.Builtin))

	case *schema.Type_Pointer:
		// FIXME(ENC-827): Handle pointers in TypeScript in a way which more technically correct without
		// making the end user experience of using a generated client worse.
		ts.writeTyp(ns, typ.Pointer.Base, numIndents)

	case *schema.Type_Literal:
		switch lit := typ.Literal.Value.(type) {
		case *schema.Literal_Str:
			ts.WriteString(ts.Quote(lit.Str))
		case *schema.Literal_Int:
			ts.WriteString(strconv.FormatInt(lit.Int, 10))
		case *schema.Literal_Float:
			ts.WriteString(strconv.FormatFloat(lit.Float, 'f', -1, 64))
		case *schema.Literal_Boolean:
			ts.WriteString(strconv.FormatBool(lit.Boolean))
		case *schema.Literal_Null:
			ts.WriteString("null")
		default:
			ts.errorf("unknown literal type %T", lit)
		}

	case *schema.Type_Union:
		for i, typ := range typ.Union.Types {
			if i > 0 {
				ts.WriteString(" | ")
			}
			ts.writeTyp(ns, typ, numIndents)
		}

	case *schema.Type_Struct:
		indent := func() {
			ts.WriteString(strings.Repeat("    ", numIndents+1))
		}

		// Filter the fields to print based on struct tags.
		fields := make([]*schema.Field, 0, len(typ.Struct.Fields))
		for _, f := range typ.Struct.Fields {
			// skip cookie fields as they are handled by the browser
			if f.Wire.GetCookie() != nil {
				continue
			}
			if encoding.IgnoreField(f) {
				continue
			}
			fields = append(fields, f)
		}

		ts.WriteString("{\n")
		for i, field := range fields {
			if field.Doc != "" {
				scanner := bufio.NewScanner(strings.NewReader(field.Doc))
				indent()
				ts.WriteString("/**\n")
				for scanner.Scan() {
					indent()
					ts.WriteString(" * ")
					ts.WriteString(scanner.Text())
					ts.WriteByte('\n')
				}
				indent()
				ts.WriteString(" */\n")
			}

			indent()
			ts.WriteString(ts.QuoteIfRequired(ts.fieldNameInStruct(field)))

			if field.Optional || ts.isRecursive(field.Typ) {
				ts.WriteString("?")
			}
			ts.WriteString(": ")
			ts.writeTyp(ns, field.Typ, numIndents+1)
			ts.WriteString("\n")

			// Add another empty line if we have a doc comment
			// and this was not the last field.
			if field.Doc != "" && i < len(fields)-1 {
				ts.WriteByte('\n')
			}
		}
		ts.WriteString(strings.Repeat("    ", numIndents))
		ts.WriteByte('}')

	case *schema.Type_TypeParameter:
		decl := ts.md.Decls[typ.TypeParameter.DeclId]
		typeParam := decl.TypeParams[typ.TypeParameter.ParamIdx]

		ts.WriteString(typeParam.Name)

	case *schema.Type_Config:
		// Config type is transparent
		ts.writeTyp(ns, typ.Config.Elem, numIndents)

	default:
		ts.errorf("unknown type %+v", reflect.TypeOf(typ))
	}
}

type bailout struct{ err error }

func (ts *typescript) errorf(format string, args ...interface{}) {
	panic(bailout{fmt.Errorf(format, args...)})
}

func (ts *typescript) handleBailout(dst *error) {
	if err := recover(); err != nil {
		if bail, ok := err.(bailout); ok {
			*dst = bail.err
		} else {
			panic(err)
		}
	}
}

func (ts *typescript) newIdentWriter(indent int) *indentWriter {
	return &indentWriter{
		w:                ts.Buffer,
		depth:            indent,
		indent:           "    ",
		firstWriteOnLine: true,
	}
}

func (ts *typescript) Quote(s string) string {
	return fmt.Sprintf("\"%s\"", strings.Replace(s, "\"", "\\\"", -1))
}

func (ts *typescript) QuoteIfRequired(s string) string {
	// If the identifier isn't purely alphanumeric, we need to add quotes.
	if !stringIsOnly(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		return ts.Quote(s)
	}
	return s
}

// Dot allows us to reference a field in a struct by its name.
func (ts *typescript) Dot(structIdent string, fieldIdent string) string {
	fieldIdent = ts.QuoteIfRequired(fieldIdent)

	if len(fieldIdent) > 0 && fieldIdent[0] == '"' {
		return fmt.Sprintf("%s[%s]", structIdent, fieldIdent)
	} else {
		return fmt.Sprintf("%s.%s", structIdent, fieldIdent)
	}
}

func (ts *typescript) Values(w *indentWriter, dict map[string]string) {
	// Work out the largest key length.
	largestKey := 0
	keys := make([]string, 0, len(dict))
	for key := range dict {
		keys = append(keys, key)
		key = ts.QuoteIfRequired(key)
		if len(key) > largestKey {
			largestKey = len(key)
		}
	}

	sort.Strings(keys)

	w.WriteString("{\n")
	{
		w := w.Indent()
		for _, key := range keys {
			ident := ts.QuoteIfRequired(key)
			w.WriteStringf("%s: %s%s,\n", ident, strings.Repeat(" ", largestKey-len(ident)), dict[key])
		}
	}
	w.WriteString("}")
}

// Regex to replace invalid characters (anything that isn't a letter, number, `_`, or `$`)
var invalidChars = regexp.MustCompile(`[^\p{L}\p{N}$_]`)

// validTSIdentifier converts a string into a valid TypeScript identifier.
func validTSIdentifier(input string) string {
	if input == "" {
		return "_"
	}
	runes := []rune(input)
	// Ensure the first character is a valid TS identifier start (letter, `_`, or `$`)
	if !unicode.IsLetter(runes[0]) && runes[0] != '_' && runes[0] != '$' {
		runes[0] = '_'
	}

	output := invalidChars.ReplaceAllString(string(runes), "_")
	return output
}

func (ts *typescript) typeName(identifier string) string {
	if ts.generatorVersion < TsExperimental {
		return validTSIdentifier(identifier)
	} else {
		return idents.Convert(identifier, idents.PascalCase)
	}
}

func (ts *typescript) memberName(identifier string) string {
	if ts.generatorVersion < TsExperimental {
		return validTSIdentifier(identifier)
	} else {
		return idents.Convert(identifier, idents.CamelCase)
	}
}

func (ts *typescript) fieldNameInStruct(field *schema.Field) string {
	name := field.Name
	if field.JsonName != "" {
		name = field.JsonName
	}
	return name
}

func (ts *typescript) isRecursive(typ *schema.Type) bool {
	// Treat recursively seen types as if they are optional
	recursiveType := false
	if n := typ.GetNamed(); n != nil {
		recursiveType = ts.typs.IsRecursiveRef(ts.currDecl.Id, n.Id)
	}
	return recursiveType
}

func (ts *typescript) writeCustomErrorType() {
	w := ts.newIdentWriter(0)

	w.WriteString(`

/**
 * APIErrorDetails represents the response from an Encore API in the case of an error
 */
interface APIErrorResponse {
    code: ErrCode
    message: string
    details?: any
}

function isAPIErrorResponse(err: any): err is APIErrorResponse {
    return (
        err !== undefined && err !== null &&
        isErrCode(err.code) &&
        typeof(err.message) === "string" &&
        (err.details === undefined || err.details === null || typeof(err.details) === "object")
    )
}

function isErrCode(code: any): code is ErrCode {
    return code !== undefined && Object.values(ErrCode).includes(code)
}

/**
 * APIError represents a structured error as returned from an Encore application.
 */
export class APIError extends Error {
    /**
     * The HTTP status code associated with the error.
     */
    public readonly status: number

    /**
     * The Encore error code
     */
    public readonly code: ErrCode

    /**
     * The error details
     */
    public readonly details?: any

    constructor(status: number, response: APIErrorResponse) {
        // extending errors causes issues after you construct them, unless you apply the following fixes
        super(response.message);

        // set error name as constructor name, make it not enumerable to keep native Error behavior
        // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/new.target#new.target_in_constructors
        Object.defineProperty(this, 'name', {
            value:        'APIError',
            enumerable:   false,
            configurable: true,
        })

        // fix the prototype chain
        if ((Object as any).setPrototypeOf == undefined) {
            (this as any).__proto__ = APIError.prototype
        } else {
            Object.setPrototypeOf(this, APIError.prototype);
        }

        // capture a stack trace
        if ((Error as any).captureStackTrace !== undefined) {
            (Error as any).captureStackTrace(this, this.constructor);
        }

        this.status = status
        this.code = response.code
        this.details = response.details
    }
}

/**
 * Typeguard allowing use of an APIError's fields'
 */
export function isAPIError(err: any): err is APIError {
    return err instanceof APIError;
}

export enum ErrCode {
    /**
     * OK indicates the operation was successful.
     */
    OK = "ok",

    /**
     * Canceled indicates the operation was canceled (typically by the caller).
     *
     * Encore will generate this error code when cancellation is requested.
     */
    Canceled = "canceled",

    /**
     * Unknown error. An example of where this error may be returned is
     * if a Status value received from another address space belongs to
     * an error-space that is not known in this address space. Also
     * errors raised by APIs that do not return enough error information
     * may be converted to this error.
     *
     * Encore will generate this error code in the above two mentioned cases.
     */
    Unknown = "unknown",

    /**
     * InvalidArgument indicates client specified an invalid argument.
     * Note that this differs from FailedPrecondition. It indicates arguments
     * that are problematic regardless of the state of the system
     * (e.g., a malformed file name).
     *
     * This error code will not be generated by the gRPC framework.
     */
    InvalidArgument = "invalid_argument",

    /**
     * DeadlineExceeded means operation expired before completion.
     * For operations that change the state of the system, this error may be
     * returned even if the operation has completed successfully. For
     * example, a successful response from a server could have been delayed
     * long enough for the deadline to expire.
     *
     * The gRPC framework will generate this error code when the deadline is
     * exceeded.
     */
    DeadlineExceeded = "deadline_exceeded",

    /**
     * NotFound means some requested entity (e.g., file or directory) was
     * not found.
     *
     * This error code will not be generated by the gRPC framework.
     */
    NotFound = "not_found",

    /**
     * AlreadyExists means an attempt to create an entity failed because one
     * already exists.
     *
     * This error code will not be generated by the gRPC framework.
     */
    AlreadyExists = "already_exists",

    /**
     * PermissionDenied indicates the caller does not have permission to
     * execute the specified operation. It must not be used for rejections
     * caused by exhausting some resource (use ResourceExhausted
     * instead for those errors). It must not be
     * used if the caller cannot be identified (use Unauthenticated
     * instead for those errors).
     *
     * This error code will not be generated by the gRPC core framework,
     * but expect authentication middleware to use it.
     */
    PermissionDenied = "permission_denied",

    /**
     * ResourceExhausted indicates some resource has been exhausted, perhaps
     * a per-user quota, or perhaps the entire file system is out of space.
     *
     * This error code will be generated by the gRPC framework in
     * out-of-memory and server overload situations, or when a message is
     * larger than the configured maximum size.
     */
    ResourceExhausted = "resource_exhausted",

    /**
     * FailedPrecondition indicates operation was rejected because the
     * system is not in a state required for the operation's execution.
     * For example, directory to be deleted may be non-empty, an rmdir
     * operation is applied to a non-directory, etc.
     *
     * A litmus test that may help a service implementor in deciding
     * between FailedPrecondition, Aborted, and Unavailable:
     *  (a) Use Unavailable if the client can retry just the failing call.
     *  (b) Use Aborted if the client should retry at a higher-level
     *      (e.g., restarting a read-modify-write sequence).
     *  (c) Use FailedPrecondition if the client should not retry until
     *      the system state has been explicitly fixed. E.g., if an "rmdir"
     *      fails because the directory is non-empty, FailedPrecondition
     *      should be returned since the client should not retry unless
     *      they have first fixed up the directory by deleting files from it.
     *  (d) Use FailedPrecondition if the client performs conditional
     *      REST Get/Update/Delete on a resource and the resource on the
     *      server does not match the condition. E.g., conflicting
     *      read-modify-write on the same resource.
     *
     * This error code will not be generated by the gRPC framework.
     */
    FailedPrecondition = "failed_precondition",

    /**
     * Aborted indicates the operation was aborted, typically due to a
     * concurrency issue like sequencer check failures, transaction aborts,
     * etc.
     *
     * See litmus test above for deciding between FailedPrecondition,
     * Aborted, and Unavailable.
     */
    Aborted = "aborted",

    /**
     * OutOfRange means operation was attempted past the valid range.
     * E.g., seeking or reading past end of file.
     *
     * Unlike InvalidArgument, this error indicates a problem that may
     * be fixed if the system state changes. For example, a 32-bit file
     * system will generate InvalidArgument if asked to read at an
     * offset that is not in the range [0,2^32-1], but it will generate
     * OutOfRange if asked to read from an offset past the current
     * file size.
     *
     * There is a fair bit of overlap between FailedPrecondition and
     * OutOfRange. We recommend using OutOfRange (the more specific
     * error) when it applies so that callers who are iterating through
     * a space can easily look for an OutOfRange error to detect when
     * they are done.
     *
     * This error code will not be generated by the gRPC framework.
     */
    OutOfRange = "out_of_range",

    /**
     * Unimplemented indicates operation is not implemented or not
     * supported/enabled in this service.
     *
     * This error code will be generated by the gRPC framework. Most
     * commonly, you will see this error code when a method implementation
     * is missing on the server. It can also be generated for unknown
     * compression algorithms or a disagreement as to whether an RPC should
     * be streaming.
     */
    Unimplemented = "unimplemented",

    /**
     * Internal errors. Means some invariants expected by underlying
     * system has been broken. If you see one of these errors,
     * something is very broken.
     *
     * This error code will be generated by the gRPC framework in several
     * internal error conditions.
     */
    Internal = "internal",

    /**
     * Unavailable indicates the service is currently unavailable.
     * This is a most likely a transient condition and may be corrected
     * by retrying with a backoff. Note that it is not always safe to retry
     * non-idempotent operations.
     *
     * See litmus test above for deciding between FailedPrecondition,
     * Aborted, and Unavailable.
     *
     * This error code will be generated by the gRPC framework during
     * abrupt shutdown of a server process or network connection.
     */
    Unavailable = "unavailable",

    /**
     * DataLoss indicates unrecoverable data loss or corruption.
     *
     * This error code will not be generated by the gRPC framework.
     */
    DataLoss = "data_loss",

    /**
     * Unauthenticated indicates the request does not have valid
     * authentication credentials for the operation.
     *
     * The gRPC framework will generate this error code when the
     * authentication metadata is invalid or a Credentials callback fails,
     * but also expect authentication middleware to generate it.
     */
    Unauthenticated = "unauthenticated",
}
`)
}

func stringIsOnly(str string, predicate func(r rune) bool) bool {
	for _, r := range str {
		if !predicate(r) {
			return false
		}
	}
	return true
}
