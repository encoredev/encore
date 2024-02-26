package clientgen

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/cockroachdb/errors"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"encr.dev/internal/clientgen/clientgentypes"
	"encr.dev/pkg/idents"

	"encr.dev/internal/version"
	"encr.dev/parser/encoding"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

/* The JavaScript generator generates code that looks like this:
class TaskServiceClient {
	public Add(params) {
		// ...
	}
}

export const task = {
    ServiceClient: TaskServiceClient
}

*/

// jsGenVersion allows us to introduce breaking changes in the generated code but behind a switch
// meaning that people with client code reliant on the old behaviour can continue to generate the
// old code.
type jsGenVersion int

const (
	// JsInitial is the originally released javascript generator
	JsInitial jsGenVersion = iota

	// JsExperimental can be used to lock experimental or uncompleted features in the generated code
	// It should always be the last item in the enum
	JsExperimental
)

const javascriptGenLatestVersion = JsExperimental - 1

type javascript struct {
	*bytes.Buffer
	md               *meta.Data
	appSlug          string
	typs             *typeRegistry
	currDecl         *schema.Decl
	generatorVersion jsGenVersion

	seenJSON           bool // true if a JSON type was seen
	seenHeaderResponse bool // true if we've seen a header used in a response object
	hasAuth            bool // true if we've seen an authentication handler
	authIsComplexType  bool // true if the auth type is a complex type
}

func (js *javascript) Version() int {
	return int(js.generatorVersion)
}

func (js *javascript) Generate(p clientgentypes.GenerateParams) (err error) {
	defer js.handleBailout(&err)

	js.Buffer = p.Buf
	js.md = p.Meta
	js.appSlug = p.AppSlug
	js.typs = getNamedTypes(p.Meta, p.Services)

	if js.md.AuthHandler != nil {
		js.hasAuth = true
		js.authIsComplexType = js.md.AuthHandler.Params.GetBuiltin() != schema.Builtin_STRING
	}

	js.WriteString("// " + doNotEditHeader() + "\n\n")

	js.WriteString("// Disable eslint, jshint, and jslint for this file.\n")
	js.WriteString("/* eslint-disable */\n")
	js.WriteString("/* jshint ignore:start */\n")
	js.WriteString("/*jslint-disable*/\n")

	seenNs := make(map[string]bool)
	js.writeClient(p.Services)
	for _, svc := range p.Meta.Svcs {
		if err := js.writeService(svc, p.Services); err != nil {
			return err
		}
		seenNs[svc.Name] = true
	}
	js.writeExtraTypes()
	if err := js.writeBaseClient(p.AppSlug); err != nil {
		return err
	}
	js.writeCustomErrorType()

	return nil
}

func (js *javascript) writeService(svc *meta.Service, set clientgentypes.ServiceSet) error {
	// Determine if we have anything worth exposing.
	// Either a public RPC or a named type.
	isIncluded := hasPublicRPC(svc) && set.Has(svc.Name)
	if !isIncluded {
		return nil
	}

	ns := svc.Name
	numIndent := 0
	indent := func() {
		js.WriteString(strings.Repeat("    ", numIndent))
	}

	fmt.Fprintf(js, "class %sServiceClient {\n", cases.Title(language.English, cases.Compact).String(js.typeName(ns)))
	numIndent++

	// Constructor
	indent()
	js.WriteString("constructor(baseClient) {\n")
	numIndent++
	indent()
	js.WriteString("this.baseClient = baseClient\n")
	numIndent--
	indent()
	js.WriteString("}\n")

	// RPCs
	for _, rpc := range svc.Rpcs {
		if rpc.AccessType == meta.RPC_PRIVATE {
			continue
		}

		js.WriteByte('\n')

		// Doc string
		if rpc.Doc != "" {
			scanner := bufio.NewScanner(strings.NewReader(rpc.Doc))
			indent()
			js.WriteString("/**\n")
			for scanner.Scan() {
				indent()
				js.WriteString(" * ")
				js.WriteString(scanner.Text())
				js.WriteByte('\n')
			}
			indent()
			js.WriteString(" */\n")
		}

		// Signature
		indent()
		fmt.Fprintf(js, "async %s(", js.memberName(rpc.Name))

		if rpc.Proto == meta.RPC_RAW {
			js.WriteString("method, ")
		}

		nParams := 0
		var rpcPath strings.Builder
		for _, s := range rpc.Path.Segments {
			rpcPath.WriteByte('/')
			if s.Type != meta.PathSegment_LITERAL {
				if nParams > 0 {
					js.WriteString(", ")
				}

				js.WriteString(js.nonReservedId(s.Value))
				if s.Type == meta.PathSegment_WILDCARD || s.Type == meta.PathSegment_FALLBACK {
					rpcPath.WriteString("${" + js.nonReservedId(s.Value) + ".map(encodeURIComponent).join(\"/\")}")
				} else {
					rpcPath.WriteString("${encodeURIComponent(" + js.nonReservedId(s.Value) + ")}")
				}
				nParams++
			} else {
				rpcPath.WriteString(s.Value)
			}
		}

		// Avoid a name collision.
		payloadName := "params"

		if rpc.RequestSchema != nil {
			if nParams > 0 {
				js.WriteString(", ")
			}
			js.WriteString(payloadName)
		} else if rpc.Proto == meta.RPC_RAW {
			if nParams > 0 {
				js.WriteString(", ")
			}
			js.WriteString("body, options")
		}

		js.WriteString(") {\n")

		err := js.rpcCallSite(js.newIdentWriter(numIndent+1), rpc, rpcPath.String())
		if err != nil {
			return errors.Wrapf(err, "unable to write RPC call site for %s.%s", rpc.ServiceName, rpc.Name)
		}

		indent()
		js.WriteString("}\n")
	}
	numIndent--
	indent()
	js.WriteString("}\n\n")

	fmt.Fprintf(js, "export const %s = {\n", js.typeName(ns))
	numIndent++
	indent()
	fmt.Fprintf(js, "ServiceClient: %sServiceClient\n", cases.Title(language.English, cases.Compact).String(js.typeName(ns)))
	numIndent--
	indent()
	js.WriteString("}\n\n")
	return nil
}

func (js *javascript) rpcCallSite(w *indentWriter, rpc *meta.RPC, rpcPath string) error {
	// Work out how we're going to encode and call this RPC
	rpcEncoding, err := encoding.DescribeRPC(js.md, rpc, &encoding.Options{SrcNameTag: "json"})
	if err != nil {
		return errors.Wrapf(err, "rpc %s", rpc.Name)
	}

	// Raw end points just pass through the request
	// and need no further code generation
	if rpc.Proto == meta.RPC_RAW {
		w.WriteStringf(
			"return this.baseClient.callAPI(method, `%s`, body, options)\n",
			rpcPath,
		)
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
				ref := js.Dot("params", field.SrcName)
				dict[field.WireFormat] = js.convertBuiltinToString(field.Type.GetBuiltin(), ref, field.Optional)
			}

			w.WriteString("const headers = makeRecord(")
			js.Values(w, dict)
			w.WriteString(")\n\n")
		}

		// Generate the query string
		if len(reqEnc.QueryParameters) > 0 {
			query = "query"

			dict := make(map[string]string)
			for _, field := range reqEnc.QueryParameters {
				if list := field.Type.GetList(); list != nil {
					dict[field.WireFormat] = js.Dot("params", field.SrcName) +
						".map((v) => " + js.convertBuiltinToString(list.Elem.GetBuiltin(), "v", field.Optional) + ")"
				} else {
					dict[field.WireFormat] = js.convertBuiltinToString(
						field.Type.GetBuiltin(),
						js.Dot("params", field.SrcName),
						field.Optional,
					)
				}
			}

			w.WriteString("const query = makeRecord(")
			js.Values(w, dict)
			w.WriteString(")\n\n")
		}

		// Generate the body
		if len(reqEnc.BodyParameters) > 0 {
			if len(reqEnc.HeaderParameters) == 0 && len(reqEnc.QueryParameters) == 0 {
				// In the simple case we can just encode the params as the body directly
				body = "JSON.stringify(params)"
			} else {
				// Else we need a new struct called "body"
				body = "JSON.stringify(body)"

				dict := make(map[string]string)
				for _, field := range reqEnc.BodyParameters {
					fieldName := field.SrcName
					dict[fieldName] = js.Dot("params", fieldName)
				}

				w.WriteString("// Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)\nconst body = ")
				js.Values(w, dict)
				w.WriteString("\n\n")
			}
		}
	}

	// Build the call to callAPI
	callAPI := fmt.Sprintf(
		"this.baseClient.callAPI(\"%s\", `%s`",
		rpcEncoding.DefaultMethod,
		rpcPath,
	)
	if body != "" || headers != "" || query != "" {
		if body == "" {
			callAPI += ", undefined"
		} else {
			callAPI += ", " + body
		}

		if headers != "" || query != "" {
			callAPI += ", {" + headers

			if headers != "" && query != "" {
				callAPI += ", "
			}

			if query != "" {
				callAPI += query
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
		w.WriteString("return await resp.json()\n")
		return nil
	}

	// Otherwise, we need to add the header fields to the response
	w.WriteString("\n//Populate the return object from the JSON body and received headers\nconst rtn = await resp.json()\n")

	for _, headerField := range respEnc.HeaderParameters {
		js.seenHeaderResponse = true
		fieldValue := fmt.Sprintf("mustBeSet(\"Header `%s`\", resp.headers.get(\"%s\"))", headerField.WireFormat, headerField.WireFormat)

		w.WriteStringf("%s = %s\n", js.Dot("rtn", headerField.SrcName), js.convertStringToBuiltin(headerField.Type.GetBuiltin(), fieldValue))
	}

	w.WriteString("return rtn\n")
	return nil
}

// nonReservedId returns the given ID, unless we have it a reserved within the client function _or_ it's a reserved Typescript keyword
func (js *javascript) nonReservedId(id string) string {
	switch id {
	// our reserved keywords (or ID's we use within the generated client functions)
	case "params", "headers", "query", "body", "resp", "rtn":
		return "_" + id

	// Javascript keywords
	// Based on https://www.w3schools.com/js/js_reserved.asp
	case "abstract", "arguments", "async", "await", "boolean", "break", "byte", "case", "catch", "char",
		"class", "const", "continue", "debugger", "default", "delete", "do", "double", "else",
		"enum", "eval", "export", "extends", "false", "final", "finally", "float", "for", "function", "get",
		"goto", "if", "implements", "import", "in", "instanceof", "int", "interface", "let", "long",
		"native", "new", "null", "of", "package", "private", "protected", "public", "require", "return",
		"short", "static", "super", "switch", "symbol", "synchronized", "this", "throw", "throws",
		"transient", "true", "try", "type", "typeof", "var", "void", "volatile", "while", "with", "yield":
		return "_" + id

	default:
		return id
	}
}

func (js *javascript) writeClient(set clientgentypes.ServiceSet) {
	w := js.newIdentWriter(0)
	w.WriteString(`
/**
 * Local is the base URL for calling the Encore application's API.
 */
export const Local = "http://localhost:4000"

/**
 * Environment returns a BaseURL for calling the cloud environment with the given name.
 */
export function Environment(name) {
    return ` + "`https://${name}-" + js.appSlug + ".encr.app`" + `
}

/**
 * PreviewEnv returns a BaseURL for calling the preview environment with the given PR number.
 */
export function PreviewEnv(pr) {
    return Environment(` + "`pr${pr}`" + `)
}

/**
 * Client is an API client for the ` + js.appSlug + ` Encore application. 
 */
export default class Client {`)

	{
		w := w.Indent()
		w.WriteString(`
/**
 * Creates a Client for calling the public and authenticated APIs of your Encore application.
 *
 * @param target  The target which the client should be configured to use. See Local and Environment for options.
 * @param options Options for the client
 */
constructor(target = "prod", options = undefined) {`)
		{
			w := w.Indent()

			if js.hasAuth && !js.authIsComplexType {
				w.WriteString(`
// Convert the old constructor parameters to a BaseURL object and a ClientOptions object
if (!target.startsWith("http://") && !target.startsWith("https://")) {
    target = Environment(target)
}

if (typeof options === "string") {
    options = { auth: options }
}

`)
			} else {
				w.WriteString("\n")
			}

			w.WriteString("const base = new BaseClient(target, options ?? {})\n")
			for _, svc := range js.md.Svcs {
				if hasPublicRPC(svc) && set.Has(svc.Name) {
					w.WriteStringf("this.%s = new %s.ServiceClient(base)\n", js.memberName(svc.Name), js.typeName(svc.Name))
				}
			}
		}

		w.WriteString("    }\n")
	}
	w.WriteString("}\n\n")
}

func (js *javascript) writeBaseClient(appSlug string) error {
	userAgent := fmt.Sprintf("%s-Generated-JS-Client (Encore/%s)", appSlug, version.Version)

	js.WriteString(`

const boundFetch = fetch.bind(this)

class BaseClient {`)
	js.WriteString(`
    constructor(baseURL, options) {
        this.baseURL = baseURL
        this.headers = {
            "Content-Type": "application/json",
        }

        // Add User-Agent header if the script is running in the server
        // because browsers do not allow setting User-Agent headers to requests
        if (typeof window === "undefined") {
            this.headers["User-Agent"] = "` + userAgent + `";
        }

        this.requestInit = options.requestInit ?? {}

        // Setup what fetch function we'll be using in the base client
        if (options.fetcher !== undefined) {
            this.fetcher = options.fetcher
        } else {
            this.fetcher = boundFetch
        }`)

	if js.hasAuth {
		js.WriteString(`

        // Setup an authentication data generator using the auth data token option
        if (options.auth !== undefined) {
            const auth = options.auth
            if (typeof auth === "function") {
                this.authGenerator = auth
            } else {
                this.authGenerator = () => auth
            }
        }
`)
	}

	js.WriteString(`
    }

    // callAPI is used by each generated API method to actually make the request
    async callAPI(method, path, body, params) {
        let { query, headers, ...rest } = params ?? {}
        const init = {
            ...this.requestInit,
            ...rest,
            method,
            body: body ?? null,
        }

        // Merge our headers with any predefined headers
        init.headers = {...this.headers, ...init.headers, ...headers}
`)
	w := js.newIdentWriter(2)

	if js.hasAuth {
		w.WriteString(`
// If authorization data generator is present, call it and add the returned data to the request
let authData`)
		w.WriteString("\n")
		w.WriteString(`if (this.authGenerator) {
    const mayBePromise = this.authGenerator()
    if (mayBePromise instanceof Promise) {
        authData = await mayBePromise
    } else {
        authData = mayBePromise
    }
}

// If we now have authentication data, add it to the request
if (authData) {
`)

		{
			w := w.Indent()
			if js.authIsComplexType {
				authData, err := encoding.DescribeAuth(js.md, js.md.AuthHandler.Params, &encoding.Options{SrcNameTag: "json"})
				if err != nil {
					return errors.Wrap(err, "unable to describe auth data")
				}

				// Write all the query string fields in
				for i, field := range authData.QueryParameters {
					// We need to ensure that the query field is not undefined
					if i == 0 {
						w.WriteString("query = query ?? {}\n")
					}

					w.WriteString("query[\"")
					w.WriteString(field.WireFormat)
					w.WriteString("\"] = ")
					if list := field.Type.GetList(); list != nil {
						w.WriteString(
							js.Dot("authData", field.SrcName) +
								".map((v) => " + js.convertBuiltinToString(list.Elem.GetBuiltin(), "v", field.Optional) + ")",
						)
					} else {
						w.WriteString(js.convertBuiltinToString(field.Type.GetBuiltin(), js.Dot("authData", field.SrcName), field.Optional))
					}
					w.WriteString("\n")
				}

				// Write all the headers
				for _, field := range authData.HeaderParameters {
					w.WriteString("init.headers[\"")
					w.WriteString(field.WireFormat)
					w.WriteString("\"] = ")
					w.WriteString(js.convertBuiltinToString(field.Type.GetBuiltin(), js.Dot("authData", field.SrcName), field.Optional))
					w.WriteString("\n")
				}
			} else {
				w.WriteString("init.headers[\"Authorization\"] = \"Bearer \" + authData\n")
			}
		}

		w.WriteString("}\n")
	}

	js.WriteString(`
        // Make the actual request
        const queryString = query ? '?' + encodeQuery(query) : ''
        const response = await this.fetcher(this.baseURL+path+queryString, init)

        // handle any error responses
        if (!response.ok) {
            // try and get the error message from the response body
            let body = { code: ErrCode.Unknown, message: ` + "`request failed: status ${response.status}`" + ` }

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

func (js *javascript) writeExtraTypes() {
	js.WriteString(`
function encodeQuery(parts) {
    const pairs = []
    for (const key in parts) {
        const val = (Array.isArray(parts[key]) ?  parts[key] : [parts[key]])
        for (const v of val) {
            pairs.push(` + "`" + `${key}=${encodeURIComponent(v)}` + "`" + `)
        }
    }
    return pairs.join("&")
}

// makeRecord takes a record and strips any undefined values from it,
// and returns the same record with a narrower type.
function makeRecord(record) {
    for (const key in record) {
        if (record[key] === undefined) {
            delete record[key]
        }
    }
    return record
}
`)

	if js.seenHeaderResponse {
		js.WriteString(`
// mustBeSet will throw an APIError with the Data Loss code if value is null or undefined
function mustBeSet(field, value) {
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

func (js *javascript) convertBuiltinToString(typ schema.Builtin, val string, isOptional bool) string {
	var code string
	switch typ {
	case schema.Builtin_STRING:
		return val
	case schema.Builtin_JSON:
		code = fmt.Sprintf("JSON.stringify(%s)", val)
	default:
		code = fmt.Sprintf("String(%s)", val)
	}

	if isOptional {
		code = fmt.Sprintf("%s === undefined ? undefined : %s", val, code)
	}
	return code
}

func (js *javascript) convertStringToBuiltin(typ schema.Builtin, val string) string {
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
		js.seenJSON = true
		return fmt.Sprintf("JSON.parse(%s)", val)
	case schema.Builtin_UUID:
		return val
	case schema.Builtin_USER_ID:
		return val
	default:
		js.errorf("unknown builtin type %v", typ)
		return "any"
	}
}

func (js *javascript) errorf(format string, args ...interface{}) {
	panic(bailout{fmt.Errorf(format, args...)})
}

func (js *javascript) handleBailout(dst *error) {
	if err := recover(); err != nil {
		if bail, ok := err.(bailout); ok {
			*dst = bail.err
		} else {
			panic(err)
		}
	}
}

func (js *javascript) newIdentWriter(indent int) *indentWriter {
	return &indentWriter{
		w:                js.Buffer,
		depth:            indent,
		indent:           "    ",
		firstWriteOnLine: true,
	}
}

func (js *javascript) Quote(s string) string {
	return fmt.Sprintf("\"%s\"", strings.Replace(s, "\"", "\\\"", -1))
}

func (js *javascript) QuoteIfRequired(s string) string {
	// If the identifier isn't purely alphanumeric, we need to add quotes.
	if !stringIsOnly(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		return js.Quote(s)
	}
	return s
}

// Dot allows us to reference a field in a struct by ijs name.
func (js *javascript) Dot(structIdent string, fieldIdent string) string {
	fieldIdent = js.QuoteIfRequired(fieldIdent)

	if len(fieldIdent) > 0 && fieldIdent[0] == '"' {
		return fmt.Sprintf("%s[%s]", structIdent, fieldIdent)
	} else {
		return fmt.Sprintf("%s.%s", structIdent, fieldIdent)
	}
}

func (js *javascript) Values(w *indentWriter, dict map[string]string) {
	// Work out the largest key length.
	largestKey := 0
	keys := make([]string, 0, len(dict))
	for key := range dict {
		keys = append(keys, key)
		key = js.QuoteIfRequired(key)
		if len(key) > largestKey {
			largestKey = len(key)
		}
	}

	sort.Strings(keys)

	w.WriteString("{\n")
	{
		w := w.Indent()
		for _, key := range keys {
			ident := js.QuoteIfRequired(key)
			w.WriteStringf("%s: %s%s,\n", ident, strings.Repeat(" ", largestKey-len(ident)), dict[key])
		}
	}
	w.WriteString("}")
}

func (js *javascript) typeName(identifier string) string {
	if js.generatorVersion < JsExperimental {
		return identifier
	} else {
		return idents.Convert(identifier, idents.PascalCase)
	}
}

func (js *javascript) memberName(identifier string) string {
	if js.generatorVersion < JsExperimental {
		return identifier
	} else {
		return idents.Convert(identifier, idents.CamelCase)
	}
}

func (js *javascript) fieldNameInStruct(field *schema.Field) string {
	name := field.Name
	if field.JsonName != "" {
		name = field.JsonName
	}
	return name
}

func (js *javascript) writeCustomErrorType() {
	w := js.newIdentWriter(0)

	w.WriteString(`

function isAPIErrorResponse(err) {
    return (
        err !== undefined && err !== null && 
        isErrCode(err.code) &&
        typeof(err.message) === "string" &&
        (err.details === undefined || err.details === null || typeof(err.details) === "object")
    )
}

function isErrCode(code) {
    return code !== undefined && Object.values(ErrCode).includes(code)
}

/**
 * APIError represents a structured error as returned from an Encore application.
 */
export class APIError extends Error {
    constructor(status, response) {
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
        if (Object.setPrototypeOf == undefined) {
            this.__proto__ = APIError.prototype
        } else {
            Object.setPrototypeOf(this, APIError.prototype);
        }
        
        // capture a stack trace
        if (Error.captureStackTrace !== undefined) {
            Error.captureStackTrace(this, this.constructor);
        }

        /**
         * The HTTP status code associated with the error.
         */
        this.status = status

        /**
         * The Encore error code
         */
        this.code = response.code

        /**
         * The error details
         */
        this.details = response.details
    }
}

/**
 * Typeguard allowing use of an APIError's fields'
 */
export function isAPIError(err) {
    return err instanceof APIError;
}

export const ErrCode = {
    /**
     * OK indicates the operation was successful.
     */
    OK: "ok",

    /**
     * Canceled indicates the operation was canceled (typically by the caller).
     *
     * Encore will generate this error code when cancellation is requested.
     */
    Canceled: "canceled",

    /**
     * Unknown error. An example of where this error may be returned is
     * if a Status value received from another address space belongs to
     * an error-space that is not known in this address space. Also
     * errors raised by APIs that do not return enough error information
     * may be converted to this error.
     *
     * Encore will generate this error code in the above two mentioned cases.
     */
    Unknown: "unknown",

    /**
     * InvalidArgument indicates client specified an invalid argument.
     * Note that this differs from FailedPrecondition. It indicates arguments
     * that are problematic regardless of the state of the system
     * (e.g., a malformed file name).
     *
     * This error code will not be generated by the gRPC framework.
     */
    InvalidArgument: "invalid_argument",

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
    DeadlineExceeded: "deadline_exceeded",

    /**
     * NotFound means some requested entity (e.g., file or directory) was
     * not found.
     *
     * This error code will not be generated by the gRPC framework.
     */
    NotFound: "not_found",

    /**
     * AlreadyExists means an attempt to create an entity failed because one
     * already exists.
     *
     * This error code will not be generated by the gRPC framework.
     */
    AlreadyExists: "already_exists",

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
    PermissionDenied: "permission_denied",

    /**
     * ResourceExhausted indicates some resource has been exhausted, perhaps
     * a per-user quota, or perhaps the entire file system is out of space.
     *
     * This error code will be generated by the gRPC framework in
     * out-of-memory and server overload situations, or when a message is
     * larger than the configured maximum size.
     */
    ResourceExhausted: "resource_exhausted",

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
    FailedPrecondition: "failed_precondition",

    /**
     * Aborted indicates the operation was aborted, typically due to a
     * concurrency issue like sequencer check failures, transaction aborts,
     * etc.
     *
     * See litmus test above for deciding between FailedPrecondition,
     * Aborted, and Unavailable.
     */
    Aborted: "aborted",

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
    OutOfRange: "out_of_range",

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
    Unimplemented: "unimplemented",

    /**
     * Internal errors. Means some invariants expected by underlying
     * system has been broken. If you see one of these errors,
     * something is very broken.
     *
     * This error code will be generated by the gRPC framework in several
     * internal error conditions.
     */
    Internal: "internal",

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
    Unavailable: "unavailable",

    /**
     * DataLoss indicates unrecoverable data loss or corruption.
     *
     * This error code will not be generated by the gRPC framework.
     */
    DataLoss: "data_loss",

    /**
     * Unauthenticated indicates the request does not have valid
     * authentication credentials for the operation.
     *
     * The gRPC framework will generate this error code when the
     * authentication metadata is invalid or a Credentials callback fails,
     * but also expect authentication middleware to generate it.
     */
    Unauthenticated: "unauthenticated"
}
`)
}
