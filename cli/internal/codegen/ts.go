package codegen

import (
	"bufio"
	"bytes"
	"fmt"
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
}

func (ts *ts) Generate(buf *bytes.Buffer, appSlug string, md *meta.Data) (err error) {
	defer ts.handleBailout(&err)

	ts.Buffer = buf
	ts.md = md
	ts.appSlug = appSlug
	ts.typs = getNamedTypes(md)

	nss := ts.typs.Namespaces()
	seenNs := make(map[string]bool)
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
	ts.writeBaseClient()

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
				fmt.Fprintf(ts, "%s: string", s.Value)
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
			ts.writeDecl(ns, rpc.RequestSchema)
		}

		ts.WriteString("): Promise<")
		if rpc.ResponseSchema != nil {
			ts.writeDecl(ns, rpc.ResponseSchema)
		} else {
			ts.WriteString("void")
		}
		ts.WriteString("> {\n")

		// Body
		numIndent++
		indent()
		method := rpc.HttpMethods[0]
		if method == "*" {
			method = "POST"
		}
		if rpc.ResponseSchema == nil {
			ts.WriteString("return this.baseClient.doVoid")
		} else {
			ts.WriteString("return this.baseClient.do<")
			ts.writeDecl(svc.Name, rpc.ResponseSchema)
			ts.WriteByte('>')
		}
		fmt.Fprintf(ts, `("%s", `+"`%s`", method, rpcPath.String())
		if rpc.RequestSchema != nil {
			ts.WriteString(", " + payloadName)
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

	// If it's a struct type, expose it as an interface;
	// other types should be type aliases.
	if st := decl.Type.GetStruct(); st != nil {
		fmt.Fprintf(ts, "    export interface %s ", decl.Name)
	} else {
		fmt.Fprintf(ts, "    export type %s = ", decl.Name)
	}
	ts.currDecl = decl
	ts.writeTyp(ns, decl.Type, 1)
	ts.WriteString("\n")
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
	ts.WriteByte('\n')

	indent()
	ts.WriteString("constructor(environment: string = \"prod\", token?: string) {\n")
	numIndent++

	indent()
	ts.WriteString("const base = new BaseClient(environment, token)\n")
	for _, svc := range ts.md.Svcs {
		if ts.hasPublicRPC(svc) {
			indent()
			fmt.Fprintf(ts, "this.%s = new %s.ServiceClient(base)\n", svc.Name, svc.Name)
		}
	}

	numIndent--
	indent()
	fmt.Fprint(ts, "}\n}\n\n")
}

func (ts *ts) writeBaseClient() {
	ts.WriteString(`class BaseClient {
    baseURL: string
    headers: {[key: string]: string}

    constructor(environment: string, token?: string) {
        this.headers = {"Content-Type": "application/json"}
        if (token !== undefined) {
            this.headers["Authorization"] = "Bearer " + token
        }
        if (environment === "local") {
            this.baseURL = "http://localhost:4060"
        } else {
            this.baseURL = ` + "`https://" + ts.appSlug + ".encoreapi.com/${environment}`" + `
        }
    }

    public async do<T>(method: string, path: string, req?: any): Promise<T> {
        let response = await fetch(this.baseURL + path, {
            method: method,
            headers: this.headers,
            body: JSON.stringify(req)
        })
        if (!response.ok) {
            let body = await response.text()
            throw new Error("request failed: " + body)
        }
        return <T>(await response.json())
    }

    public async doVoid(method: string, path: string, req?: any): Promise<void> {
        let response = await fetch(this.baseURL + path, {
            method: method,
            headers: this.headers,
            body: JSON.stringify(req)
        })
        if (!response.ok) {
            let body = await response.text()
            throw new Error("request failed: " + body)
        }
        await response.text()
    }
}
`)
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
			t = "object"
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
		for i, field := range typ.Struct.Fields {
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
			if field.Doc != "" && i < len(typ.Struct.Fields)-1 {
				ts.WriteByte('\n')
			}
		}
		ts.WriteString(strings.Repeat("    ", numIndents))
		ts.WriteByte('}')
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
