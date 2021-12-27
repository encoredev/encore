package codegen

import (
	"bufio"
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"

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

type ts struct {
	*bytes.Buffer
	md       *meta.Data
	appSlug  string
	typs     *typeRegistry
	currDecl *schema.Decl

	seenJSON bool // true if a JSON type was seen
}

func (ts *ts) Generate(buf *bytes.Buffer, appSlug string, md *meta.Data) (err error) {
	defer ts.handleBailout(&err)

	ts.Buffer = buf
	ts.md = md
	ts.appSlug = appSlug
	ts.typs = getNamedTypes(md)

	nss := ts.typs.Namespaces()
	seenNs := make(map[string]bool)
	ts.writeGenericTypes()
	ts.writeClient()
	for _, svc := range md.Svcs {
		ts.writeService(svc)
		seenNs[svc.Name] = true
	}
	for _, ns := range nss {
		if !seenNs[ns] {
			ts.writeNamespace(ns)
		}
	}
	ts.writeExtraTypes()
	ts.writeHelperMethods()

	return nil
}

func (ts *ts) hasPublicRPC(svc *meta.Service) bool {
	for _, rpc := range svc.Rpcs {
		if rpc.AccessType != meta.RPC_PRIVATE {
			return true
		}
	}
	return false
}

func (ts *ts) writeService(svc *meta.Service) {
	// Determine if we have anything worth exposing.
	// Either a public RPC or a named type.
	publicRPC := ts.hasPublicRPC(svc)
	decls := ts.typs.Decls(svc.Name)
	if !publicRPC && len(decls) == 0 {
		return
	}

	ns := svc.Name
	fmt.Fprintf(ts, "export namespace %s {\n", ns)

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
		return
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
	ts.WriteString("private client: Client\n\n")
	indent()
	ts.WriteString("constructor(client: Client) {\n")
	numIndent++
	indent()
	ts.WriteString("this.client = client\n")
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
		fmt.Fprintf(ts, "public %s(", rpc.Name)

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
			ts.WriteString("req: any")
		}

		ts.WriteString(") {\n")

		// Body
		numIndent++
		indent()
		method := rpc.HttpMethods[0]
		if method == "*" {
			method = "POST"
		}

		var (
			methodHasBody bool
			hasQuery      bool
		)
		switch method {
		case "GET", "HEAD", "DELETE":
			methodHasBody = false
			if rpc.RequestSchema != nil && rpc.RequestSchema.GetNamed() != nil {
				decl := ts.md.Decls[rpc.RequestSchema.GetNamed().Id]
				for _, f := range decl.Type.GetStruct().Fields {
					if f.QueryStringName == "-" || f.JsonName == "-" {
						continue
					}
					fieldName := f.JsonName
					if fieldName == "" {
						fieldName = f.Name
					}
					if !hasQuery {
						hasQuery = true
						ts.WriteString("const query: any[] = [\n")
						numIndent++
					}
					indent()
					ts.WriteString("\"" + f.QueryStringName + "\", params." + fieldName + ",\n")
				}
				if hasQuery {
					rpcPath.WriteString("?${encodeQuery(query)}")
					numIndent--
					indent()
					ts.WriteString("]\n")
					indent()
				}
			}
		default:
			methodHasBody = true
		}
		if rpc.Proto == meta.RPC_RAW {
			ts.WriteString("return this.client.doRaw")
		} else if rpc.ResponseSchema == nil {
			ts.WriteString("return this.client.doVoid")
		} else {
			ts.WriteString("return this.client.do<")
			ts.writeTyp(svc.Name, rpc.ResponseSchema, 0)
			ts.WriteByte('>')
		}
		fmt.Fprintf(ts, `("%s", `+"`%s`", method, rpcPath.String())
		if rpc.RequestSchema != nil && methodHasBody {
			ts.WriteString(", " + payloadName)
		} else if rpc.Proto == meta.RPC_RAW {
			ts.WriteString(", req")
		}
		ts.WriteString(")\n")
		numIndent--
		indent()
		ts.WriteString("}\n")
	}
	numIndent--
	indent()
	ts.WriteString("}\n}\n\n")
}

func (ts *ts) writeNamespace(ns string) {
	decls := ts.typs.Decls(ns)
	if len(decls) == 0 {
		return
	}

	fmt.Fprintf(ts, "export namespace %s {\n", ns)
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

func (ts *ts) writeDeclDef(ns string, decl *schema.Decl) {
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
		fmt.Fprintf(ts, "    export interface %s%s ", decl.Name, typeParams.String())
	} else {
		fmt.Fprintf(ts, "    export type %s%s = ", decl.Name, typeParams.String())
	}
	ts.currDecl = decl
	ts.writeTyp(ns, decl.Type, 1)
	ts.WriteString("\n")
}

func (ts *ts) writeGenericTypes() {
	ts.WriteString("export interface ErrorResponse {\n")
	ts.WriteString("    code: number,\n")
	ts.WriteString("    message: string,\n")
	ts.WriteString("    details: any\n")
	ts.WriteString("}\n\n")
	ts.WriteString("export type Result<T> = { data: T } | { error: ErrorResponse }\n\n")
}

func (ts *ts) writeClient() {
	ts.WriteString("export default class Client {\n")

	numIndent := 1
	indent := func() {
		ts.WriteString(strings.Repeat("    ", numIndent))
	}

	for _, svc := range ts.md.Svcs {
		if ts.hasPublicRPC(svc) {
			indent()
			fmt.Fprintf(ts, "%s: %s.ServiceClient\n", svc.Name, svc.Name)
		}
	}
	ts.writeClientMethods()
	ts.WriteByte('\n')
	indent()
	ts.WriteString("protected baseURL: string\n")
	ts.WriteByte('\n')
	indent()
	ts.WriteString("constructor(environment: string = \"prod\", public token?: string) {\n")
	numIndent++
	indent()
	ts.WriteString(`if (environment.startsWith('http://') || environment.startsWith('https://')) {`)
	ts.WriteByte('\n')
	numIndent++
	indent()
	ts.WriteString(`this.baseURL = environment`)
	ts.WriteByte('\n')
	numIndent--
	indent()
	ts.WriteString(`} else {`)
	ts.WriteByte('\n')
	numIndent++
	indent()
	ts.WriteString(`this.baseURL = environment === "local" ? "http://localhost:4000" : ` + "`https://" + ts.appSlug + ".encoreapi.com/${environment}`")
	ts.WriteByte('\n')
	numIndent--
	indent()
	ts.WriteString(`}`)
	ts.WriteByte('\n')
	for _, svc := range ts.md.Svcs {
		if ts.hasPublicRPC(svc) {
			indent()
			fmt.Fprintf(ts, "this.%s = new %s.ServiceClient(this)\n", svc.Name, svc.Name)
		}
	}

	numIndent--
	indent()
	fmt.Fprint(ts, "}\n}\n\n")
}

func (ts *ts) writeClientMethods() {
	ts.WriteString(`
    public async doRaw(method: string, path: string, body?: any): Promise<Response> {
        const headers: Record<string, string> = { "Content-Type": "application/json" }
        if (this.token) {
            headers["Authorization"] = "Bearer " + this.token
        }
        return fetch(this.baseURL + path, {
            method,
            headers,
            body
        })
    }

    public async do<T>(method: string, path: string, req?: any): Promise<Result<T>> {
        try {
            const response = await this.doRaw(method, path, req !== undefined ? JSON.stringify(req) : undefined)
            if (!response.ok) {
                const error = <ErrorResponse>(await response.json())
                return { error }
            }
            return { data: <T>(await response.json().catch(_ => null)) }
        } catch (error) {
            return {
                error: <ErrorResponse>{
                    code: -1,
                    message: error.message
                }
            }
        }
    }

    public async doVoid(method: string, path: string, req?: any): Promise<ErrorResponse | null> {
        try {
            const response = await this.doRaw(method, path, req !== undefined ? JSON.stringify(req) : undefined)
            if (!response.ok) {
                const error = <ErrorResponse>(await response.json())
                return error
            }
            return null

        } catch (error) {
            return <ErrorResponse>{
                code: -1,
                message: error.message

            }
        }
    }`)
	ts.WriteByte('\n')
}
func (ts *ts) writeHelperMethods() {
	ts.WriteString(
		`function encodeQuery(parts: any[]): string {
    const pairs = []
    for (let i = 0; i < parts.length; i += 2) {
        const key = parts[i]
        let val = parts[i + 1]
        if (!Array.isArray(val)) {
            val = [val]
        }
        for (const v of val) {
            pairs.push(` + "`${key}=${encodeURIComponent(v)}`" + `)
        }
    }
    return pairs.join("&")
}
`)
}

func (ts *ts) writeExtraTypes() {
	if ts.seenJSON {
		ts.WriteString(`// JSONValue represents an arbitrary JSON value.
export type JSONValue = string | number | boolean | null | JSONValue[] | { [key: string]: JSONValue }

`)
	}
}

func (ts *ts) writeDecl(ns string, decl *schema.Decl) {
	if decl.Loc.PkgName != ns {
		ts.WriteString(decl.Loc.PkgName + ".")
	}
	ts.WriteString(decl.Name)
}

func (ts *ts) writeTyp(ns string, typ *schema.Type, numIndents int) {
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
		t := ""
		switch typ.Builtin {
		case schema.Builtin_ANY:
			t = "any"
		case schema.Builtin_BOOL:
			t = "boolean"
		case schema.Builtin_INT, schema.Builtin_INT8, schema.Builtin_INT16, schema.Builtin_INT32, schema.Builtin_INT64,
			schema.Builtin_UINT, schema.Builtin_UINT8, schema.Builtin_UINT16, schema.Builtin_UINT32, schema.Builtin_UINT64,
			schema.Builtin_FLOAT32, schema.Builtin_FLOAT64:
			t = "number"
		case schema.Builtin_STRING:
			t = "string"
		case schema.Builtin_BYTES:
			t = "string" // TODO
		case schema.Builtin_TIME:
			t = "string" // TODO
		case schema.Builtin_JSON:
			t = "JSONValue"
			ts.seenJSON = true
		case schema.Builtin_UUID:
			t = "string"
		case schema.Builtin_USER_ID:
			t = "string"
		default:
			ts.errorf("unknown builtin type %v", typ.Builtin)
		}
		ts.WriteString(t)

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
			name := field.Name
			if js := field.JsonName; js != "" {
				name = js
			}
			ts.WriteString(name)

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

func (ts *ts) errorf(format string, args ...interface{}) {
	panic(bailout{fmt.Errorf(format, args...)})
}

func (ts *ts) handleBailout(dst *error) {
	if err := recover(); err != nil {
		if bail, ok := err.(bailout); ok {
			*dst = bail.err
		} else {
			panic(err)
		}
	}
}
