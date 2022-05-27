import {ModeSpec, ModeSpecOptions} from "codemirror"
import React, {Fragment} from "react"
import CM from "~c/api/cm/CM"
import {APIMeta} from "./api"
import {
    Builtin,
    Decl, DescribedField,
    Field,
    FieldLocation,
    fieldNameAndLocation,
    ListType,
    locationDescription,
    MapType,
    NamedType,
    splitFieldsByLocation,
    StructType,
    Type,
    TypeParameterRef
} from "./schema"

export type Dialect = "go" | "typescript" | "json" | "table";

interface Props {
    meta: APIMeta;
    type: Type;
    dialect: Dialect;
    method: string;
    asResponse?: boolean;
}

export default class extends React.Component<Props> {
    render() {
        const d = dialects[this.props.dialect](this.props.meta)
        return d.render(this.props.type, this.props.method, this.props.asResponse ?? false)
    }
}

abstract class DialectIface {
    readonly meta: APIMeta;

    constructor(meta: APIMeta) {
        this.meta = meta
    }

    abstract render(d: Type, method: string, asResponse: boolean): JSX.Element
}

/** This Text based class allows us simply to build a dialect from raw text and use CodeMirror to render it */
abstract class TextBasedDialect extends DialectIface {
    private readonly codeMirrorMode: string | ModeSpec<ModeSpecOptions>
    seenDecls: Set<number>;
    typeArgumentStack: Type[][];
    buf: string[];
    level: number;
    method: string;
    asResponse: boolean;

    protected constructor(meta: APIMeta, codeMirrorMode: string | ModeSpec<ModeSpecOptions>) {
        super(meta)
        this.codeMirrorMode = codeMirrorMode
        this.seenDecls = new Set<number>()
        this.typeArgumentStack = []
        this.buf = []
        this.level = 0
        this.method = 'POST'
        this.asResponse = false
    }

    render(d: Type, method: string, asResponse: boolean): JSX.Element {
        this.method = method
        this.asResponse = asResponse
        const srcCode = this.renderAsText(d)

        return <CM cfg={{
            value:    srcCode,
            readOnly: true,
            theme:    "encore",
            mode:     this.codeMirrorMode,
        }}
                   key={srcCode}
                   noShadow={true}
        />
    }

    renderAsText(d: Type): string {
        this.writeType(d, true)

        return this.buf.join("")
    }

    protected writeType(t: Type, topLevel?: boolean, altValue?: boolean) {
        t.struct ? this.renderStruct(t.struct, topLevel) :
            t.map ? this.renderMap(t.map) :
                t.list ? this.renderList(t.list) :
                    t.builtin ? this.renderBuiltin(t.builtin, altValue) :
                        t.named ? this.renderNamed(t.named, topLevel) :
                            t.type_parameter ? this.renderTypeParameter(t.type_parameter) :
                                this.write("<unknown type>")
    }

    protected renderNamed(t: NamedType, topLevel?: boolean) {
        if (this.seenDecls.has(t.id)) {
            this.writeSeenDecl(this.meta.decls[t.id])
            return
        }

        // Add the decl to our map while recursing to avoid infinite recursion.
        this.seenDecls.add(t.id)
        const decl = this.meta.decls[t.id]
        this.typeArgumentStack.push(t.type_arguments)

        this.writeType(decl.type, topLevel)

        this.typeArgumentStack.pop()
        this.seenDecls.delete(t.id)
    }

    protected renderTypeParameter(t: TypeParameterRef) {
        const typeArguments = this.typeArgumentStack[this.typeArgumentStack.length - 1]
        if (typeArguments === undefined || typeArguments.length <= t.param_idx) {
            this.write("?")
        } else {
            this.writeType(typeArguments[t.param_idx])
        }
    }

    protected abstract writeSeenDecl(decl: Decl): void

    protected abstract renderStruct(t: StructType, topLevel?: boolean): void

    protected abstract renderMap(t: MapType): void

    protected abstract renderList(t: ListType): void

    protected abstract renderBuiltin(t: Builtin, altValue?: boolean): void

    protected indent() {
        this.write(" ".repeat(this.level * 4))
    }

    protected write(...strs: string[]) {
        for (const s of strs) {
            this.buf.push(s)
        }
    }

    protected writeln(...strs: string[]) {
        this.write(...strs)
        this.write("\n")
    }
}

class GoDialect extends TextBasedDialect {
    constructor(meta: APIMeta) {
        super(meta, "go")
        this.seenDecls = new Set()
    }

    writeSeenDecl(decl: Decl) {
        this.write(`*${decl.loc.pkg_name}.${decl.name}`)
    }

    renderStruct(t: StructType) {
        this.writeln("struct {")
        this.level++

        // Calculate the longest field name so we can align the types
        const longestFieldName = t.fields.reduce<number>((previous: number, current: Field) => {
            if (current.name.length > previous) {
                return current.name.length
            }

            return previous
        }, 0)

        t.fields.map(f => {
            this.indent()
            this.write(f.name)
            this.write(" ".repeat(longestFieldName - f.name.length + 1))
            this.writeType(f.typ)

            if (f.raw_tag) {
                this.write(" `", f.raw_tag, "`")
            }

            this.writeln()
        })

        this.level--

        this.indent()
        this.write("}")
    }

    renderMap(t: MapType,) {
        this.write("map[")
        this.writeType(t.key)
        this.write("]")
        this.writeType(t.value)
    }

    renderList(t: ListType) {
        this.write("[]")
        this.writeType(t.elem)
    }

    renderBuiltin(t: Builtin) {
        switch (t) {
            case Builtin.ANY:
                return this.write("interface{}")
            case Builtin.BOOL:
                return this.write("bool")
            case Builtin.INT:
                return this.write("int")
            case Builtin.INT8:
                return this.write("int8")
            case Builtin.INT16:
                return this.write("int16")
            case Builtin.INT32:
                return this.write("int32")
            case Builtin.INT64:
                return this.write("int64")
            case Builtin.UINT:
                return this.write("uint")
            case Builtin.UINT8:
                return this.write("uint8")
            case Builtin.UINT16:
                return this.write("uint16")
            case Builtin.UINT32:
                return this.write("uint32")
            case Builtin.UINT64:
                return this.write("uint64")
            case Builtin.FLOAT32:
                return this.write("float32")
            case Builtin.FLOAT64:
                return this.write("float64")
            case Builtin.STRING:
                return this.write("string")
            case Builtin.BYTES:
                return this.write("[]byte")
            case Builtin.TIME:
                return this.write("time.Time")
            case Builtin.UUID:
                return this.write("uuid.UUID")
            case Builtin.JSON:
                return this.write("json.RawMessage")
            case Builtin.USER_ID:
                return this.write("auth.UID")
            case Builtin.UNRECOGNIZED:
                return this.write("<unknown>")
        }

        return unreachableUnknownType(t)
    }
}

class TypescriptDialect extends TextBasedDialect {
    constructor(meta: APIMeta) {
        super(meta, {name: "javascript", typescript: true})
    }

    renderStruct(t: StructType) {
        this.writeln("{")
        this.level++

        t.fields.map(f => {
            this.indent()
            this.write(f.json_name !== "" ? f.json_name : f.name)
            this.write(": ")
            this.writeType(f.typ)

            if (f.optional) {
                this.write(" | undefined")
            }

            this.writeln(";")
        })

        this.level--
        this.indent()
        this.write("}")
    }

    renderMap(t: MapType) {
        this.write("{ [key: ")
        this.writeType(t.key)
        this.write("]: ")
        this.writeType(t.value)
        this.write("}")
    }

    renderList(t: ListType) {
        this.writeType(t.elem)
        this.write("[]")
    }

    renderBuiltin(t: Builtin) {
        switch (t) {
            case Builtin.ANY:
                return this.write("any")
            case Builtin.BOOL:
                return this.write("boolean")
            case Builtin.INT:
                return this.write("int")
            case Builtin.INT8:
                return this.write("int8")
            case Builtin.INT16:
                return this.write("int16")
            case Builtin.INT32:
                return this.write("int32")
            case Builtin.INT64:
                return this.write("int64")
            case Builtin.UINT:
                return this.write("uint")
            case Builtin.UINT8:
                return this.write("uint8")
            case Builtin.UINT16:
                return this.write("uint16")
            case Builtin.UINT32:
                return this.write("uint32")
            case Builtin.UINT64:
                return this.write("uint64")
            case Builtin.FLOAT32:
                return this.write("float32")
            case Builtin.FLOAT64:
                return this.write("float64")
            case Builtin.STRING:
                return this.write("string")
            case Builtin.BYTES:
                return this.write("[]byte")
            case Builtin.TIME:
                return this.write("Time")
            case Builtin.UUID:
                return this.write("UUID")
            case Builtin.JSON:
                return this.write("any")
            case Builtin.USER_ID:
                return this.write("UserID")
            case Builtin.UNRECOGNIZED:
                return this.write("<unknown>")
        }

        return unreachableUnknownType(t)
    }

    protected writeSeenDecl(decl: Decl): void {
        this.write("null")
    }
}

export class JSONDialect extends TextBasedDialect {
    constructor(md: APIMeta) {
        super(md, {name: "javascript", json: true})
    }

    writeSeenDecl(decl: Decl) {
        this.write("null")
    }

    public structBits(t: StructType, asGoStruct: boolean): [string, string, string] {
        const writeObj = (fields: DescribedField[]): string => {
            const oldBuf = this.buf
            this.buf = []

            this.level++
            for (let i = 0; i < fields.length; i++) {
                const f = fields[i]
                this.writeln()
                this.indent()
                this.write(`"${asGoStruct ? f.SrcName : f.name}": `)
                this.writeType(f.typ)
                if (i < (fields.length - 1)) {
                    this.write(",")
                }
            }
            this.level--
            this.indent()

            const toReturn = this.buf.join("")
            this.buf = oldBuf
            return toReturn
        }

        const fields = splitFieldsByLocation(t, this.method, this.asResponse)

        let query = ''
        let headers = ''
        let json = ''

        if (fields[FieldLocation.Query].length > 0) {
            if (asGoStruct) {
                query = writeObj(fields[FieldLocation.Query])
            } else {
                const oldBuf = this.buf
                this.buf = []

                this.write("?")

                let firstField = true
                for (const field of fields[FieldLocation.Query]) {
                    if (firstField) {
                        firstField = false
                    } else {
                        this.write("&")
                    }

                    this.write(field.name, "=")

                    if (field.typ.builtin) {
                        this.renderBuiltin(field.typ.builtin, false, true)
                    } else if (field.typ.list) {
                        this.renderBuiltin(field.typ.list.elem.builtin!, false, true)

                        // show it's a list by duplicating :-)
                        this.write("&", field.name, "=")
                        this.renderBuiltin(field.typ.list.elem.builtin!, true, true)
                    }
                }

                query = this.buf.join("")
                this.buf = oldBuf
            }

        }

        if (fields[FieldLocation.Header].length > 0) {
            headers = writeObj(fields[FieldLocation.Header])
        }

        if (fields[FieldLocation.Body].length > 0) {
            json = writeObj(fields[FieldLocation.Body])
        }

        return [query, headers, json]
    }

    protected renderStruct(t: StructType, topLevel?: boolean) {
        if (topLevel) {
            const [query, headers, json] = this.structBits(t, false)

            let previousSection = false
            if (query) {
                this.write("// Query String\n", query)
                previousSection = true
            }

            if (headers) {
                if (previousSection) {
                    this.write("\n\n")
                }

                this.write("// HTTP Headers\n{", headers, "\n}")
                previousSection = true
            }

            if (json) {
                if (previousSection) {
                    this.write("\n\n// JSON Payload\n")
                }

                this.write("{", json, "\n}")
            }

            return
        }

        this.writeln("{")
        this.level++
        for (let i = 0; i < t.fields.length; i++) {
            const f = t.fields[i]
            this.indent()
            this.write(`"${f.json_name !== "" ? f.json_name : f.name}": `)
            this.writeType(f.typ)
            if (i < (t.fields.length - 1)) {
                this.write(",")
            }
            this.writeln()
        }
        this.level--
        this.indent()
        this.write("}")

        const toReturn = this.buf.join("")
    }

    protected renderMap(t: MapType) {
        this.writeln("{")
        this.level++
        this.indent()
        this.writeType(t.key)
        this.write(": ")
        this.writeType(t.value)
        this.writeln()
        this.write("}")
    }

    protected renderList(t: ListType) {
        this.write("[")
        this.writeType(t.elem, false, false)
        this.write(", ")
        this.writeType(t.elem, false, true)
        this.write("]")
    }

    protected renderBuiltin(t: Builtin, alt?: boolean, urlEncode?: boolean) {
        let write = (s: string) => {
            if (!urlEncode) {
                return this.write(s)
            }

            if (s[0] === '"' && s[s.length - 1] === '"') {
                s = s.substring(1, s.length - 1)
            }

            return this.write(encodeURIComponent(s))
        }

        switch (t) {
            case Builtin.ANY:
                return write("<any data>")
            case Builtin.BOOL:
                return write(alt ? "false" : "true")
            case Builtin.INT:
            case Builtin.INT8:
            case Builtin.INT16:
            case Builtin.INT32:
            case Builtin.INT64:
            case Builtin.UINT:
            case Builtin.UINT8:
            case Builtin.UINT16:
            case Builtin.UINT32:
            case Builtin.UINT64:
                return write(alt ? "2" : "1")
            case Builtin.FLOAT32:
            case Builtin.FLOAT64:
                return write(alt ? "42.9" : "2.3")
            case Builtin.STRING:
                return write(alt ? "\"another string\"" : "\"some string\"")
            case Builtin.BYTES:
                return write("\"YmFzZTY0Cg==\"") // base64
            case Builtin.TIME:
                return write("\"2009-11-10T23:00:00Z\"")
            case Builtin.UUID:
                return write("\"7d42f515-3517-4e76-be13-30880443546f\"")
            case Builtin.JSON:
                return write("{\"some json data\": true}")
            case Builtin.USER_ID:
                return write("\"userID\"")
            case Builtin.UNRECOGNIZED:
                return write("<unknown>")
        }

        return unreachableUnknownType(t)
    }
}

class TableDialect extends DialectIface {
    typeArgumentStack: Type[][] = [];

    render(d: Type, method: string, asResponse: boolean) {
        if (!d?.named) {
            throw new Error("TableDialect can only rendered named structs")
        }

        const st = this.meta.decls[d.named.id].type.struct
        if (!st) {
            throw new Error("TableDialect can only render named structs")
        }

        this.typeArgumentStack.push(d.named.type_arguments)
        return this.renderStruct(st, 0, method, asResponse)
    }

    renderStruct(t: StructType, level: number, method: string, asResponse: boolean): JSX.Element {
        return (
            <div className={level !== 0 ? "rounded-sm border-gray-200" : ""}>
                {t.fields.map((f, i) => {
                    const [name, location] = fieldNameAndLocation(f, method, asResponse)

                    if (location === FieldLocation.UnusedField) {
                        return <Fragment key={f.name}/>
                    }

                    return <div key={f.name} className={i > 0 ? "border-t border-gray-200 mt-1 pt-1" : ""}>
                        <div className="flex leading-6 font-mono">
                            <div className="font-bold text-gray-900 text-sm">
                                {f.name}
                            </div>
                            <div className="ml-2 text-xs text-gray-500 flex-grow p-0.5">{this.describeType(f.typ)}</div>
                            <div className="text-xs text-gray-700 bg-gray-100 rounded text-center p-1 cursor-help"
                                 title={locationDescription(name, location)}>{location}</div>
                        </div>
                        <div className="flex">
                            {f.doc !== "" ? (
                                <div className="text-sm text-gray-700 flex-grow">{f.doc}</div>
                            ) : (
                                <div className="text-xs text-gray-400 flex-grow">No description.</div>
                            )}
                            <div className="text-xs font-mono text-gray-500" title={"The encoded field name on the wire"}>
                                {name}
                            </div>
                        </div>
                    </div>
                })}
            </div>
        )
    }

    describeType(t: Type): string {
        return (
            t.struct ? "struct" :
                t.map ? "map" :
                    t.list ? "list of " + this.describeType(t.list.elem) :
                        t.builtin ? this.describeBuiltin(t.builtin) :
                            t.named ? this.describeNamed(t.named) :
                                t.type_parameter ? this.describeTypeParameter(t.type_parameter) :
                                    "<unknown>"
        )
    }

    describeTypeParameter(t: TypeParameterRef): string {
        const typeArgument = this.typeArgumentStack[this.typeArgumentStack.length - 1][t.param_idx]
        return this.describeType(typeArgument)
    }


    describeBuiltin(t: Builtin): string {
        switch (t) {
            case Builtin.ANY:
                return "<any>"
            case Builtin.BOOL:
                return "boolean"
            case Builtin.INT:
                return "int"
            case Builtin.INT8:
                return "int"
            case Builtin.INT16:
                return "int"
            case Builtin.INT32:
                return "int"
            case Builtin.INT64:
                return "int"
            case Builtin.UINT:
                return "uint"
            case Builtin.UINT8:
                return "uint"
            case Builtin.UINT16:
                return "uint"
            case Builtin.UINT32:
                return "uint"
            case Builtin.UINT64:
                return "uint"
            case Builtin.FLOAT32:
                return "float"
            case Builtin.FLOAT64:
                return "float"
            case Builtin.STRING:
                return "string"
            case Builtin.BYTES:
                return "bytes"
            case Builtin.TIME:
                return "RFC 3339-formatted timestamp"
            case Builtin.UUID:
                return "UUID"
            case Builtin.JSON:
                return "arbitrary JSON"
            case Builtin.USER_ID:
                return "User ID"
            case Builtin.UNRECOGNIZED:
                return "<unknown>"
        }

        return unreachableUnknownType(t)
    }

    describeNamed(named: NamedType): string {
        const decl = this.meta.decls[named.id]

        let types = ""
        if (named.type_arguments.length > 0) {
            types = "["

            for (let i = 0; i < named.type_arguments.length; i++) {
                if (i > 0) {
                    types += ", "
                }

                types += this.describeType(named.type_arguments[i])
            }

            types += "]"
        }

        return decl.loc.pkg_name + "." + decl.name + types
    }
}

const dialects: { [key in Dialect]: (meta: APIMeta) => DialectIface } = {
    "go":         (meta) => new GoDialect(meta),
    "typescript": (meta) => new TypescriptDialect(meta),
    "json":       (meta) => new JSONDialect(meta),
    "table":      (meta) => new TableDialect(meta),
}

// This function serves two purposes
//
// 1. If we ever hit it at runtime; we'll return "<unknown>" to be rendered
// 2. If we have a switch statement on an enum without a default, and return from each case then if we miss one of the
//    enum options, we'll get a compile error if we try and pass that case as the parameter x to this function
export function unreachableUnknownType(_: never): string {
    return "<unknown>";
}
