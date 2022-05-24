package codegen

import (
	"bufio"
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/cockroachdb/errors"

	"encr.dev/cli/internal/version"
	"encr.dev/parser/encoding"
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

	seenJSON        bool // true if a JSON type was seen
	seenQueryString bool // true if a query string was seen
}

func (ts *typescript) Version() int {
	return int(ts.generatorVersion)
}

func (ts *typescript) Generate(buf *bytes.Buffer, appSlug string, md *meta.Data) (err error) {
	defer ts.handleBailout(&err)

	ts.Buffer = buf
	ts.md = md
	ts.appSlug = appSlug
	ts.typs = getNamedTypes(md)

	ts.WriteString("// " + doNotEditHeader() + "\n")

	nss := ts.typs.Namespaces()
	seenNs := make(map[string]bool)
	ts.writeClient()
	for _, svc := range md.Svcs {
		if err := ts.writeService(svc); err != nil {
			return err
		}
		seenNs[svc.Name] = true
	}
	for _, ns := range nss {
		if !seenNs[ns] {
			ts.writeNamespace(ns)
		}
	}
	ts.writeExtraTypes()
	ts.writeBaseClient(appSlug)
	ts.writeCustomErrorType()

	return nil
}

func (ts *typescript) writeService(svc *meta.Service) error {
	// Determine if we have anything worth exposing.
	// Either a public RPC or a named type.
	publicRPC := hasPublicRPC(svc)
	decls := ts.typs.Decls(svc.Name)
	if !publicRPC && len(decls) == 0 {
		return nil
	}

	ns := svc.Name
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

	if !publicRPC {
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
	numIndent--
	indent()
	ts.WriteString("}\n")

	// RPCs
	for _, rpc := range svc.Rpcs {
		if rpc.AccessType == meta.RPC_PRIVATE {
			continue
		}

		ts.WriteByte('\n')

		// Doc string
		if rpc.Doc != "" {
			scanner := bufio.NewScanner(strings.NewReader(rpc.Doc))
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

		if rpc.Proto == meta.RPC_RAW {
			ts.WriteString("method: ")
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
			ts.WriteString(", ")
		}

		nParams := 0
		var rpcPath strings.Builder
		paramNames := make(map[string]bool)
		for _, s := range rpc.Path.Segments {
			rpcPath.WriteByte('/')
			if s.Type != meta.PathSegment_LITERAL {
				if nParams > 0 {
					ts.WriteString(", ")
				}
				ts.WriteString(s.Value)
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
				paramNames[s.Value] = true
				rpcPath.WriteString("${" + s.Value + "}")
				nParams++
			} else {
				rpcPath.WriteString(s.Value)
			}
		}

		// Avoid a name collision.
		payloadName := "params"
		for paramNames[payloadName] {
			payloadName = "_" + payloadName
		}

		if rpc.RequestSchema != nil {
			if nParams > 0 {
				ts.WriteString(", ")
			}
			ts.WriteString(payloadName + ": ")
			ts.writeTyp(ns, rpc.RequestSchema, 0)
		} else if rpc.Proto == meta.RPC_RAW {
			if nParams > 0 {
				ts.WriteString(", ")
			}
			ts.WriteString("body?: BodyInit, options?: CallParameters")
		}

		ts.WriteString("): Promise<")
		if rpc.ResponseSchema != nil {
			ts.writeTyp(ns, rpc.ResponseSchema, 0)
		} else if rpc.Proto == meta.RPC_RAW {
			ts.WriteString("Response")
		} else {
			ts.WriteString("void")
		}
		ts.WriteString("> {\n")

		err := ts.rpcCallSite(ns, ts.newIdentWriter(numIndent+1), rpc, rpcPath.String())
		if err != nil {
			return errors.Wrapf(err, "unable to write RPC call site for %s.%s", rpc.ServiceName, rpc.Name)
		}

		indent()
		ts.WriteString("}\n")
	}
	numIndent--
	indent()
	ts.WriteString("}\n}\n\n")
	return nil
}

func (ts *typescript) rpcCallSite(ns string, w *indentWriter, rpc *meta.RPC, rpcPath string) error {
	// Work out how we're going to encode and call this RPC
	rpcEncoding, err := encoding.DescribeRPC(ts.md, rpc)
	if err != nil {
		return errors.Wrapf(err, "rpc %s", rpc.Name)
	}

	// Raw end points just pass through the request
	// and need no further code generation
	if rpc.Proto == meta.RPC_RAW {
		w.WriteStringf(
			"return this.baseClient.callAPI(method, `%s`, %s, body, options)\n",
			rpcPath,
			strconv.FormatBool(rpc.AccessType == meta.RPC_AUTH),
		)
		return nil
	}

	// Work out how we encode the Request Schema
	headers := ""
	body := ""

	if rpc.RequestSchema != nil {
		headerFields, queryFields, bodyFields, err := toFieldLists(rpcEncoding.DefaultRequestEncoding.Fields)
		if err != nil {
			return err
		}

		if len(headerFields) > 0 || len(queryFields) > 0 {
			w.WriteString("// Convert our params into the objects we need for the request\n")
		}

		// Generate the headers
		if len(headerFields) > 0 {
			headers = "headers"

			dict := make(map[string]string)
			for _, field := range headerFields {
				ref := ts.Dot("params", ts.fieldNameInStruct(field.Field))
				dict[field.Name] = ts.convertBuiltinToString(field.Field.Typ.GetBuiltin(), ref)
			}

			w.WriteString("const headers: Record<string, string> = ")
			ts.Values(w, dict)
			w.WriteString("\n")
		}

		// Generate the query string
		if len(queryFields) > 0 {
			ts.seenQueryString = true
			rpcPath += "?${encodeQuery(query)}"

			dict := make(map[string]string)
			for _, field := range queryFields {
				dict[field.Name] = ts.Dot("params", ts.fieldNameInStruct(field.Field))
			}

			w.WriteString("const query: Record<string, any> = ")
			ts.Values(w, dict)
			w.WriteString("\n")
		}

		// Generate the body
		if len(bodyFields) > 0 {
			if len(headerFields) == 0 && len(queryFields) == 0 {
				// In the simple case we can just encode the params as the body directly
				body = "JSON.stringify(params)"
			} else {
				// Else we need a new struct called "body"
				body = "JSON.stringify(body)"

				dict := make(map[string]string)
				for _, field := range bodyFields {
					fieldName := ts.fieldNameInStruct(field.Field)
					dict[fieldName] = ts.Dot("params", fieldName)
				}

				w.WriteString("// Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)\nconst body: Record<string, any> = ")
				ts.Values(w, dict)
				w.WriteString("\n")
			}
		}
	}

	// Build the call to callAPI
	callAPI := fmt.Sprintf(
		"this.baseClient.callAPI(\"%s\", `%s`, %s",
		rpcEncoding.DefaultRequestEncoding.HTTPMethods[0],
		rpcPath,
		strconv.FormatBool(rpc.AccessType == meta.RPC_AUTH),
	)
	if body != "" || headers != "" {
		if body == "" {
			callAPI += ", undefined"
		} else {
			callAPI += ", " + body
		}

		if headers != "" {
			callAPI += ", {" + headers + "}"
		}
	}
	callAPI += ")"

	// If there's no response schema, we can just return the call to the API directly
	if rpc.ResponseSchema == nil {
		w.WriteStringf("await %s\n", callAPI)
		return nil
	}

	w.WriteStringf("// Now make the actual call to the API\nconst resp = await %s\n", callAPI)

	headerFields, queryFields, _, err := toFieldLists(rpcEncoding.ResponseEncoding.Fields)
	if err != nil {
		return err
	}
	if len(queryFields) > 0 {
		return errors.New("expected no query fields in response encoding")
	}

	// If we don't need to do anything with the body, we can just return the response
	if len(headerFields) == 0 {
		w.WriteString("return await resp.json() as ")
		ts.writeTyp(ns, rpc.ResponseSchema, 0)
		w.WriteString("\n")
		return nil
	}

	// Otherwise, we need to add the header fields to the response
	w.WriteString("\n//Populate the return object from the JSON body and received headers\nconst rtn = await resp.json() as ")
	ts.writeTyp(ns, rpc.ResponseSchema, 0)
	w.WriteString("\n")

	for _, headerField := range headerFields {
		fieldValue := fmt.Sprintf("resp.headers.get(\"%s\")", headerField.Name)

		w.WriteStringf("%s = %s\n", ts.Dot("rtn", ts.fieldNameInStruct(headerField.Field)), ts.convertStringToBuiltin(headerField.Field.Typ.GetBuiltin(), fieldValue))
	}

	w.WriteString("return rtn\n")
	return nil
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

func (ts *typescript) writeClient() {
	w := ts.newIdentWriter(0)
	w.WriteString(`
/**
 * BaseURL is the base URL for calling the Encore application's API.
 */
export type BaseURL = string

export const Local: BaseURL = "http://localhost:4000"

/**
 * Environment returns a BaseURL for calling the cloud environment with the given name.
 */
export function Environment(name: string): BaseURL {
    return ` + "`https://${name}-" + ts.appSlug + ".encr.app`" + `
}

/**
 * Client is an API client for the ` + ts.appSlug + ` Encore application. 
 */
export default class Client {
`)

	{
		w := w.Indent()

		for _, svc := range ts.md.Svcs {
			if hasPublicRPC(svc) {
				w.WriteStringf("public readonly %s: %s.ServiceClient\n", ts.memberName(svc.Name), ts.typeName(svc.Name))
			}
		}
		w.WriteString("\n")

		w.WriteString(`
/**
 * @deprecated This constructor is deprecated, and you should move to using BaseURL with an Options object
 */
constructor(target?: string, token?: string)

/**
 * Creates a Client for calling the public and authenticated APIs of your Encore application.
 *
 * @param target  The target which the client should be configured to use. See Local and Environment for options.
 * @param options Options for the client
 */
constructor(target: BaseURL, options?: ClientOptions)
`)
		w.WriteString("constructor(target: string | BaseURL = \"prod\", opts?: string | ClientOptions) {\n")
		{
			w := w.Indent()

			w.WriteString(`
// Convert the old constructor parameters to a BaseURL object and a ClientOptions object
if (!target.startsWith("http://") && !target.startsWith("https://")) {
    target = Environment(target)
}

if (typeof opts === "string") {
	opts = { bearerToken: opts }
} else {
	opts ??= {}
}

`)

			w.WriteString("const base = new BaseClient(target, opts)\n")
			for _, svc := range ts.md.Svcs {
				if hasPublicRPC(svc) {
					w.WriteStringf("this.%s = new %s.ServiceClient(base)\n", ts.memberName(svc.Name), ts.typeName(svc.Name))
				}
			}
		}
		w.WriteString("}\n")
	}
	w.WriteString("}\n")

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

    /**
     * Allows you to set the auth token to be used for each request
     * either by passing in a static token string or by passing in a function
     * which returns the auth token.
     *
     * These tokens will be sent as bearer tokens in the Authorization header.
     */
    bearerToken?: string | TokenGenerator
}

`)
}

func (ts *typescript) writeBaseClient(appSlug string) {
	userAgent := fmt.Sprintf("%s-Generated-TS-Client (Encore/%s)", appSlug, version.Version)

	ts.WriteString(`
// CallParameters is the type of the parameters to a method call, but require headers to be a Record type
type CallParameters = Omit<RequestInit, "method" | "body"> & { headers?: Record<string, string> }

// TokenGenerator is a function that returns a token
export type TokenGenerator = () => string

// A fetcher is the prototype for the inbuilt Fetch function
export type Fetcher = (input: RequestInfo, init?: RequestInit) => Promise<Response>;

class BaseClient {
    readonly baseURL: string
    readonly fetcher: Fetcher
    readonly headers: Record<string, string>
    readonly tokenGenerator?: TokenGenerator

    constructor(baseURL: string, options: ClientOptions) {
        this.baseURL = baseURL
        this.headers = {
            "Content-Type": "application/json",
            "User-Agent":   "` + userAgent + `",
        }

        // Setup what fetch function we'll be using in the base client
        if (options.fetcher !== undefined) {
            this.fetcher = options.fetcher
        } else {
            this.fetcher = fetch
        }

        // Setup a token generator using the bearer token option
        if (options.bearerToken !== undefined) {
            const token = options.bearerToken
            if (typeof token === "string") {
                this.tokenGenerator = () => token
            } else {
                this.tokenGenerator = token
            }
        }
    }

    // callAPI is used by each generated API method to actually make the request
    public async callAPI(method: string, path: string, requiresAuth: boolean, body?: BodyInit, params?: CallParameters): Promise<Response> {
        const init: RequestInit = {
            method,
            body,
            ...(params ?? {}),
        }

        // Merge our headers with any predefined headers
        init.headers = {...this.headers, ...init.headers}

        let bearerToken: string | undefined

        // If an authorization token generator is present, call it and add the returned token to the request
        if (this.tokenGenerator) {
            bearerToken = this.tokenGenerator()
        }

        // If we now have a bearer token, add it to the request
        if (bearerToken) {
            init.headers["Authorization"] = "Bearer " + bearerToken
        } else if (requiresAuth) {
            // The API called requires authorization, but we don't have a token generator, or it provided a blank token value
            throw new APIError(400, { code: ErrCode.Unauthenticated, message: 'Authorization token required for this API, but none provided.' })
        }

        // Make the actual request
        const response = await this.fetcher(this.baseURL + path, init)

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
                body.message += ": " + e.toString()
            }

            throw new APIError(response.status, body)
        }

        return response
    }
}`)
}

func (ts *typescript) writeExtraTypes() {
	if ts.seenJSON {
		ts.WriteString(`// JSONValue represents an arbitrary JSON value.
export type JSONValue = string | number | boolean | null | JSONValue[] | {[key: string]: JSONValue}
`)
	}

	if ts.seenQueryString {
		ts.WriteString(`

function encodeQuery(parts: Record<string, any>): string {
    const pairs = []
    for (let key in parts) {
        let val = parts[key]
        if (!Array.isArray(val)) {
            val = [val]
        }
        for (const v of val) {
            pairs.push(` + "`" + `${key}=${encodeURIComponent(v)}` + "`" + `)
        }
    }
    return pairs.join("&")
}`)
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
	default:
		ts.errorf("unknown builtin type %v", typ)
		return "any"
	}
}

func (ts *typescript) convertBuiltinToString(typ schema.Builtin, val string) string {
	switch typ {
	case schema.Builtin_STRING:
		return val
	default:
		return fmt.Sprintf("String(%s)", val)
	}
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
		ts.writeTyp(ns, elem, numIndents)
		ts.WriteString("[]")

	case *schema.Type_Map:
		ts.WriteString("{ [key: ")
		ts.writeTyp(ns, typ.Map.Key, numIndents)
		ts.WriteString("]: ")
		ts.writeTyp(ns, typ.Map.Value, numIndents)
		ts.WriteString(" }")

	case *schema.Type_Builtin:
		ts.WriteString(ts.builtinType(typ.Builtin))

	case *schema.Type_Struct:
		indent := func() {
			ts.WriteString(strings.Repeat("    ", numIndents+1))
		}
		ts.WriteString("{\n")

		// Filter the fields to print based on struct tags.
		fields := make([]*schema.Field, 0, len(typ.Struct.Fields))
		for _, f := range typ.Struct.Fields {
			if f.JsonName == "-" {
				continue
			}
			fields = append(fields, f)
		}

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

			// Treat recursively seen types as if they are optional
			recursiveType := false
			if n := field.Typ.GetNamed(); n != nil {
				recursiveType = ts.typs.IsRecursiveRef(ts.currDecl.Id, n.Id)
			}
			if field.Optional || recursiveType {
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
	w.WriteString("}\n")
}

func (ts *typescript) typeName(identifier string) string {
	if ts.generatorVersion < TsExperimental {
		return identifier
	} else {
		return convertIdentifierTo(identifier, PascalCase)
	}
}

func (ts *typescript) memberName(identifier string) string {
	if ts.generatorVersion < TsExperimental {
		return identifier
	} else {
		return convertIdentifierTo(identifier, CamelCase)
	}
}

func (ts *typescript) fieldNameInStruct(field *schema.Field) string {
	name := field.Name
	if field.JsonName != "" {
		name = field.JsonName
	}
	return name
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
        typeof(err.code) === "number" &&
        typeof(err.message) === "string" &&
        (err.details === undefined || err.details === null || typeof(err.details) === "object")
    )
}

/**
 * APIError represents a structured error as returned from an Encore application.
 */
export class APIError extends Error {
    /**
     * The name of this error class
     */
    public readonly name: 'APIError'

    /**
     * The HTTP status code associated with the error.
     */
    public readonly status: number

    /**
     * The Encore error code
     */
    public readonly code: ErrCode

    /**
     * The error message
     */
    public readonly message: string

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
