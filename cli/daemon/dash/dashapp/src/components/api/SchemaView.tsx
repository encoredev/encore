import {BuiltinType, Decl, Field, ListType, MapType, NamedType, StructType, Type} from "./schema";
import React from "react";
import {APIMeta} from "./api";

export type Dialect = "go" | "typescript" | "json" | "table";

interface Props {
  meta: APIMeta;
  decl: Decl;
  dialect: Dialect;
}

export default class extends React.Component<Props> {
  render() {
    const d = dialects[this.props.dialect](this.props.meta)
    return d.render(this.props.decl)
  }
}

abstract class DialectIface {
  meta: APIMeta;
  constructor(meta: APIMeta) {
    this.meta = meta
  }

  abstract render(d: Decl): JSX.Element;
}

class GoDialect extends DialectIface {
  seenDecls: Set<number>;
  constructor(meta: APIMeta) {
    super(meta)
    this.seenDecls = new Set()
  }

  render(d: Decl) {
    this.seenDecls.add(d.id)
    const res = <>type {d.name} {this.renderType(d.type, 0)}</>
    this.seenDecls.delete(d.id)
    return res
  }

  renderType(t: Type, level: number) {
    return <span className="whitespace-no-wrap">{(
      t.struct ? this.renderStruct(t.struct, level) :
      t.map ? this.renderMap(t.map, level) :
      t.list ? this.renderList(t.list, level) :
      t.builtin ? this.renderBuiltin(t.builtin, level) :
      t.named ? this.renderNamed(t.named, level)
      : "<unknown>"
    )}</span>
  }

  renderNamed(t: NamedType, level: number) {
    const decl = this.meta.decls[t.id]
    if (this.seenDecls.has(t.id)) {
      return <>{`*${decl.loc.pkg_name}.${decl.name}`}</>
    }

    // Mark this decl as seen for the duration of this call
    // to avoid infinite recursion.
    this.seenDecls.add(t.id)
    const res = this.renderType(decl.type, level)
    this.seenDecls.delete(t.id)
    return res
  }

  renderStruct(t: StructType, level: number) {
    return <>
      {"struct {"}
      <div style={{paddingLeft: "4ch"}}>
        {t.fields.map(f =>
          <div key={f.name}>
            {f.name} {this.renderType(f.typ, level+1)}
            {this.renderTag(f)}
          </div>
        )}
      </div>
      <div>{"}"}</div>
    </>
  }

  renderMap(t: MapType, level: number) {
    return <>
      {"map["}
      {this.renderType(t.key, level)}
      {"]"}
      {this.renderType(t.value, level)}
    </>
  }

  renderList(t: ListType, level: number) {
    return <>
      {"[]"}
      {this.renderType(t.elem, level)}
    </>
  }

  renderBuiltin(t: BuiltinType, level: number) {
    switch (t) {
    case BuiltinType.Any: return "interface{}"
    case BuiltinType.Bool: return "bool"
    case BuiltinType.Int: return "int"
    case BuiltinType.Int8: return "int8"
    case BuiltinType.Int16: return "int16"
    case BuiltinType.Int32: return "int32"
    case BuiltinType.Int64: return "int64"
    case BuiltinType.Uint: return "uint"
    case BuiltinType.Uint8: return "uint8"
    case BuiltinType.Uint16: return "uint16"
    case BuiltinType.Uint32: return "uint32"
    case BuiltinType.Uint64: return "uint64"
    case BuiltinType.Float32: return "float32"
    case BuiltinType.Float64: return "float64"
    case BuiltinType.String: return "string"
    case BuiltinType.Bytes: return "[]byte"
    case BuiltinType.Time: return "time.Time"
    case BuiltinType.UUID: return "uuid.UUID"
    case BuiltinType.USER_ID: return "auth.UID"
    case BuiltinType.JSON: return "json.RawMessage"
    default: return "unknown"
    }
  }

  renderTag(f: Field): string | null {
    let parts = []
    if (f.optional) {
      parts.push(`encore:"optional"`)
    }
    if (f.json_name !== "") {
      parts.push(`json:"${f.json_name}"`)
    }
    if (parts.length === 0) {
      return null
    }
    return " `" + parts.join(" ") + "`"
  }
}

class TypescriptDialect extends DialectIface {
  seenDecls: Set<number>;
  constructor(meta: APIMeta) {
    super(meta)
    this.seenDecls = new Set()
  }

  render(d: Decl) {
    return this.renderType(d.type, 0)
  }

  renderType(t: Type, level: number) {
    return <span className="whitespace-no-wrap">{(
      t.struct ? this.renderStruct(t.struct, level) :
      t.map ? this.renderMap(t.map, level) :
      t.list ? this.renderList(t.list, level) :
      t.builtin ? this.renderBuiltin(t.builtin, level) :
      t.named ? this.renderNamed(t.named, level)
      : "<unknown>"
    )}</span>
  }

  renderNamed(t: NamedType, level: number) {
    if (this.seenDecls.has(t.id)) {
      return <>null</>
    }
    const decl = this.meta.decls[t.id]

    // Mark this decl as seen for the duration of this call
    // to avoid infinite recursion.
    this.seenDecls.add(t.id)
    const res = this.renderType(decl.type, level)
    this.seenDecls.delete(t.id)
    return res
  }

  renderStruct(t: StructType, level: number) {
    return <>
      {"{"}
      <div style={{paddingLeft: "2ch"}}>
        {t.fields.map(f =>
          <div key={f.name}>{f.json_name !== "" ? f.json_name : f.name}: {this.renderType(f.typ, level+1)};</div>
        )}
      </div>
      {"}"}
    </>
  }

  renderMap(t: MapType, level: number) {
    return <>
      {"{ [key: "}
      {this.renderType(t.key, level)}
      {"]: "}
      {this.renderType(t.value, level)}
      {"}"}
    </>
  }

  renderList(t: ListType, level: number) {
    return <>
      {this.renderType(t.elem, level)}
      {"[]"}
    </>
  }

  renderBuiltin(t: BuiltinType, level: number) {
    switch (t) {
    case BuiltinType.Any: return "any"
    case BuiltinType.Bool: return "boolean"
    case BuiltinType.Int8: return "int8"
    case BuiltinType.Int16: return "int16"
    case BuiltinType.Int32: return "int32"
    case BuiltinType.Int64: return "int64"
    case BuiltinType.Uint8: return "uint8"
    case BuiltinType.Uint16: return "uint16"
    case BuiltinType.Uint32: return "uint32"
    case BuiltinType.Uint64: return "uint64"
    case BuiltinType.Float32: return "float32"
    case BuiltinType.Float64: return "float64"
    case BuiltinType.String: return "string"
    case BuiltinType.Bytes: return "[]byte"
    case BuiltinType.Time: return "Time"
    case BuiltinType.UUID: return "UUID"
    case BuiltinType.JSON: return "any"
    case BuiltinType.USER_ID: return "UserID"
    default: return "unknown"
    }
  }
}

class JSONDialect extends DialectIface {
  seenDecls: Set<number>;
  constructor(meta: APIMeta) {
    super(meta)
    this.seenDecls = new Set()
  }

  render(d: Decl) {
    return this.renderType(d.type, 0)
  }

  renderType(t: Type, level: number) {
    return <span className="whitespace-no-wrap">{(
      t.struct ? this.renderStruct(t.struct, level) :
      t.map ? this.renderMap(t.map, level) :
      t.list ? this.renderList(t.list, level) :
      t.builtin ? this.renderBuiltin(t.builtin, level) :
      t.named ? this.renderNamed(t.named, level)
      : "<unknown>"
    )}</span>
  }

  renderNamed(t: NamedType, level: number) {
    if (this.seenDecls.has(t.id)) {
      return <>null</>
    }
    const decl = this.meta.decls[t.id]

    // Mark this decl as seen for the duration of this call
    // to avoid infinite recursion.
    this.seenDecls.add(t.id)
    const res = this.renderType(decl.type, level)
    this.seenDecls.delete(t.id)
    return res
  }

  renderStruct(t: StructType, level: number) {
    return <>
      {"{"}
      <div style={{paddingLeft: "2ch"}}>
        {t.fields.map((f, i) =>
          <div key={f.name}>
            "{f.json_name !== "" ? f.json_name : f.name}": {this.renderType(f.typ, level+1)}
            {
              /* Render trailing comma if it's not the last key */
              (i < (t.fields.length-1)) ? "," : ""
            }
          </div>
        )}
      </div>
      {"}"}
    </>
  }

  renderMap(t: MapType, level: number) {
    return <>
      {"{"}
      {this.renderType(t.key, level)}
      {": "}
      {this.renderType(t.value, level)}
      {"}"}
    </>
  }

  renderList(t: ListType, level: number) {
    return <>
      {"["}
      {this.renderType(t.elem, level)}
      {"]"}
    </>
  }

  renderBuiltin(t: BuiltinType, level: number) {
    switch (t) {
    case BuiltinType.Any: return "<any>"
    case BuiltinType.Bool: return "true"
    case BuiltinType.Int: return "1"
    case BuiltinType.Int8: return "1"
    case BuiltinType.Int16: return "1"
    case BuiltinType.Int32: return "1"
    case BuiltinType.Int64: return "1"
    case BuiltinType.Uint: return "1"
    case BuiltinType.Uint8: return "1"
    case BuiltinType.Uint16: return "1"
    case BuiltinType.Uint32: return "1"
    case BuiltinType.Uint64: return "1"
    case BuiltinType.Float32: return "2.3"
    case BuiltinType.Float64: return "2.3"
    case BuiltinType.String: return "\"some-string\""
    case BuiltinType.Bytes: return "\"base64-encoded-bytes\""
    case BuiltinType.Time: return "\"2009-11-10T23:00:00Z\""
    case BuiltinType.UUID: return "\"7d42f515-3517-4e76-be13-30880443546f\""
    case BuiltinType.JSON: return "{\"some-json-data\": true}"
    case BuiltinType.USER_ID: return "\"some-user-id\""
    default: return "<unknown>"
    }
  }
}

class TableDialect extends DialectIface {
  render(d: Decl) {
    const st = d.type.struct
    if (!st) {
      throw new Error("TableDialect can only render named structs")
    }
    return this.renderStruct(st, 0)
  }

  renderStruct(t: StructType, level: number): JSX.Element {
    return (
      <div className={level !== 0 ? "rounded-sm border-gray-200" : ""}>
          {t.fields.map((f, i) =>
            <div key={f.name} className={i > 0 ? "border-t border-gray-200" : ""}>
              <div className="flex leading-6 font-mono">
                <div className="font-bold text-gray-900 text-sm">
                  {f.name}
                </div>
                <div className="ml-2 text-xs text-gray-500">{this.describeType(f.typ)}</div>
              </div>
              {f.doc !== "" ? (
                <div className="text-sm text-gray-700">{f.doc}</div>
              ) : (
                <div className="text-xs text-gray-400">No description.</div>
              )}
            </div>
          )}
      </div>
    )
  }

  describeType(t: Type): string {
    return (
      t.struct ? "struct" :
      t.map ? "map" :
      t.list ? "list of " + this.describeType(t.list.elem) :
      t.builtin ? this.describeBuiltin(t.builtin) :
      t.named ? this.describeNamed(t.named)
      : "<unknown>"
    )
  }


  describeBuiltin(t: BuiltinType): string {
    switch (t) {
    case BuiltinType.Any: return "<any>"
    case BuiltinType.Bool: return "boolean"
    case BuiltinType.Int: return "int"
    case BuiltinType.Int8: return "int"
    case BuiltinType.Int16: return "int"
    case BuiltinType.Int32: return "int"
    case BuiltinType.Int64: return "int"
    case BuiltinType.Uint: return "uint"
    case BuiltinType.Uint8: return "uint"
    case BuiltinType.Uint16: return "uint"
    case BuiltinType.Uint32: return "uint"
    case BuiltinType.Uint64: return "uint"
    case BuiltinType.Float32: return "float"
    case BuiltinType.Float64: return "float"
    case BuiltinType.String: return "string"
    case BuiltinType.Bytes: return "bytes"
    case BuiltinType.Time: return "RFC 3339-formatted timestamp"
    case BuiltinType.UUID: return "UUID"
    case BuiltinType.JSON: return "unspecified JSON"
    case BuiltinType.USER_ID: return "User ID"
    default: return "<unknown>"
    }
  }

  describeNamed(named: NamedType): string {
    const decl = this.meta.decls[named.id]
    return decl.loc.pkg_name + "." + decl.name
  }
}

const dialects: { [key in Dialect]: (meta: APIMeta) => DialectIface} = {
  "go": (meta) => new GoDialect(meta),
  "typescript": (meta) => new TypescriptDialect(meta),
  "json": (meta) => new JSONDialect(meta),
  "table": (meta) => new TableDialect(meta),
}
