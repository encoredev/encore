import { ModeSpec, ModeSpecOptions } from "codemirror";
import React, { Fragment } from "react";
import CM from "~c/api/cm/CM";
import { APIMeta, PathSegment_SegmentType, RPC, Service } from "./api";
import {
  Builtin,
  Decl,
  DescribedField,
  Field,
  FieldLocation,
  fieldNameAndLocation,
  ListType,
  locationDescription,
  MapType,
  NamedType,
  Pointer,
  rpcHasBody,
  splitFieldsByLocation,
  StructType,
  Type,
  TypeParameterRef,
} from "./schema";
import HJSON from "hjson";

export type Dialect = "go" | "typescript" | "json" | "curl" | "table";

interface Props {
  meta: APIMeta;
  type: Type;
  dialect: Dialect;
  service: Service;
  rpc: RPC;
  method: string;
  asResponse?: boolean;
}

export default class extends React.Component<Props> {
  render() {
    const d = dialects[this.props.dialect](this.props.meta);
    return d.render(
      this.props.type,
      this.props.service,
      this.props.rpc,
      this.props.method,
      this.props.asResponse ?? false
    );
  }
}

abstract class DialectIface {
  readonly meta: APIMeta;

  constructor(meta: APIMeta) {
    this.meta = meta;
  }

  abstract render(
    d: Type,
    service: Service,
    rpc: RPC,
    method: string,
    asResponse: boolean
  ): JSX.Element;
}

/** This Text based class allows us simply to build a dialect from raw text and use CodeMirror to render it */
abstract class TextBasedDialect extends DialectIface {
  private readonly codeMirrorMode: string | ModeSpec<ModeSpecOptions>;
  seenDecls: Set<number>;
  typeArgumentStack: Type[][];
  buf: string[];
  level: number;
  method: string;
  asResponse: boolean;
  service?: Service;
  rpc?: RPC;

  protected constructor(meta: APIMeta, codeMirrorMode: string | ModeSpec<ModeSpecOptions>) {
    super(meta);
    this.codeMirrorMode = codeMirrorMode;
    this.seenDecls = new Set<number>();
    this.typeArgumentStack = [];
    this.buf = [];
    this.level = 0;
    this.method = "POST";
    this.asResponse = false;
  }

  render(d: Type, service: Service, rpc: RPC, method: string, asResponse: boolean): JSX.Element {
    this.service = service;
    this.rpc = rpc;
    this.method = method;
    this.asResponse = asResponse;
    const srcCode = this.renderAsText(d);

    return (
      <CM
        cfg={{
          value: srcCode,
          readOnly: true,
          theme: "encore",
          mode: this.codeMirrorMode,
        }}
        key={srcCode}
        noShadow={true}
      />
    );
  }

  renderAsText(d: Type): string {
    this.writeType(d, true);

    return this.buf.join("");
  }

  protected writeType(t: Type, topLevel?: boolean, altValue?: boolean) {
    t.struct
      ? this.renderStruct(t.struct, topLevel)
      : t.map
      ? this.renderMap(t.map)
      : t.list
      ? this.renderList(t.list)
      : t.builtin
      ? this.renderBuiltin(t.builtin, altValue)
      : t.named
      ? this.renderNamed(t.named, topLevel)
      : t.type_parameter
      ? this.renderTypeParameter(t.type_parameter)
      : t.pointer
      ? this.renderPointer(t.pointer)
      : this.write("<unknown type>");
  }

  protected renderNamed(t: NamedType, topLevel?: boolean) {
    if (this.seenDecls.has(t.id)) {
      this.writeSeenDecl(this.meta.decls[t.id]);
      return;
    }

    // Add the decl to our map while recursing to avoid infinite recursion.
    this.seenDecls.add(t.id);
    const decl = this.meta.decls[t.id];
    this.typeArgumentStack.push(t.type_arguments);

    this.writeType(decl.type, topLevel);

    this.typeArgumentStack.pop();
    this.seenDecls.delete(t.id);
  }

  protected renderTypeParameter(t: TypeParameterRef) {
    const typeArguments = this.typeArgumentStack[this.typeArgumentStack.length - 1];
    if (typeArguments === undefined || typeArguments.length <= t.param_idx) {
      this.write("?");
    } else {
      this.writeType(typeArguments[t.param_idx]);
    }
  }

  protected abstract writeSeenDecl(decl: Decl): void;

  protected abstract renderStruct(t: StructType, topLevel?: boolean): void;

  protected abstract renderMap(t: MapType): void;

  protected abstract renderList(t: ListType): void;

  protected abstract renderPointer(t: Pointer, topLevel?: boolean): void;

  protected abstract renderBuiltin(t: Builtin, altValue?: boolean): void;

  protected indent() {
    this.write(" ".repeat(this.level * 4));
  }

  protected write(...strs: string[]) {
    for (const s of strs) {
      this.buf.push(s);
    }
  }

  protected writeln(...strs: string[]) {
    this.write(...strs);
    this.write("\n");
  }
}

class GoDialect extends TextBasedDialect {
  constructor(meta: APIMeta) {
    super(meta, "go");
    this.seenDecls = new Set();
  }

  writeSeenDecl(decl: Decl) {
    this.write(`*${decl.loc.pkg_name}.${decl.name}`);
  }

  renderStruct(t: StructType, topLevel?: boolean) {
    this.writeln("struct {");
    this.level++;

    const typeAsString = (typ: Type): string => {
      const oldBuf = this.buf;
      this.buf = [];
      this.writeType(typ);
      const type = this.buf.join("");
      this.buf = oldBuf;

      return type;
    };

    // Calculate the longest field name so we can align the types
    const longestFieldName = t.fields.reduce<number>((previous: number, current: Field) => {
      if (current.name.length > previous) {
        return current.name.length;
      }

      return previous;
    }, 0);

    const longestSingleLineType = t.fields.reduce<number>((previous: number, current: Field) => {
      const type = typeAsString(current.typ);
      if (type.indexOf("\n") < 0 && type.length > previous) {
        return type.length;
      }

      return previous;
    }, 0);

    t.fields.map((f) => {
      if (topLevel) {
        const [_, location] = fieldNameAndLocation(f, this.method, this.asResponse);
        if (location === FieldLocation.UnusedField) {
          return;
        }
      } else if (f.json_name == "-") {
        return;
      }

      this.indent();
      this.write(f.name);
      this.write(" ".repeat(longestFieldName - f.name.length + 1));

      const type = typeAsString(f.typ);
      this.write(type);
      if (type.indexOf("\n") < 0) {
        this.write(" ".repeat(Math.max(longestSingleLineType - type.length, 0)));
      }

      if (f.raw_tag) {
        this.write(" `", f.raw_tag, "`");
      }

      this.writeln();
    });

    this.level--;

    this.indent();
    this.write("}");
  }

  renderMap(t: MapType) {
    this.write("map[");
    this.writeType(t.key);
    this.write("]");
    this.writeType(t.value);
  }

  renderList(t: ListType) {
    this.write("[]");
    this.writeType(t.elem);
  }

  renderPointer(t: Pointer, topLevel?: boolean) {
    this.write("*");
    this.writeType(t.base, topLevel);
  }

  renderBuiltin(t: Builtin) {
    switch (t) {
      case Builtin.ANY:
        return this.write("interface{}");
      case Builtin.BOOL:
        return this.write("bool");
      case Builtin.INT:
        return this.write("int");
      case Builtin.INT8:
        return this.write("int8");
      case Builtin.INT16:
        return this.write("int16");
      case Builtin.INT32:
        return this.write("int32");
      case Builtin.INT64:
        return this.write("int64");
      case Builtin.UINT:
        return this.write("uint");
      case Builtin.UINT8:
        return this.write("uint8");
      case Builtin.UINT16:
        return this.write("uint16");
      case Builtin.UINT32:
        return this.write("uint32");
      case Builtin.UINT64:
        return this.write("uint64");
      case Builtin.FLOAT32:
        return this.write("float32");
      case Builtin.FLOAT64:
        return this.write("float64");
      case Builtin.STRING:
        return this.write("string");
      case Builtin.BYTES:
        return this.write("[]byte");
      case Builtin.TIME:
        return this.write("time.Time");
      case Builtin.UUID:
        return this.write("uuid.UUID");
      case Builtin.JSON:
        return this.write("json.RawMessage");
      case Builtin.USER_ID:
        return this.write("auth.UID");
      case Builtin.UNRECOGNIZED:
        return this.write("<unknown>");
    }

    return unreachableUnknownType(t);
  }
}

class TypescriptDialect extends TextBasedDialect {
  constructor(meta: APIMeta) {
    super(meta, { name: "javascript", typescript: true });
  }

  renderStruct(t: StructType) {
    this.writeln("{");
    this.level++;

    t.fields.map((f) => {
      this.indent();
      this.write(f.json_name !== "" ? f.json_name : f.name);
      this.write(": ");
      this.writeType(f.typ);

      if (f.optional) {
        this.write(" | undefined");
      }

      this.writeln(";");
    });

    this.level--;
    this.indent();
    this.write("}");
  }

  renderMap(t: MapType) {
    this.write("{ [key: ");
    this.writeType(t.key);
    this.write("]: ");
    this.writeType(t.value);
    this.write("}");
  }

  renderList(t: ListType, topLevel?: boolean) {
    this.writeType(t.elem, topLevel);
    this.write("[]");
  }

  renderPointer(t: Pointer) {
    this.writeType(t.base);
  }

  renderBuiltin(t: Builtin) {
    switch (t) {
      case Builtin.ANY:
        return this.write("any");
      case Builtin.BOOL:
        return this.write("boolean");
      case Builtin.INT:
        return this.write("int");
      case Builtin.INT8:
        return this.write("int8");
      case Builtin.INT16:
        return this.write("int16");
      case Builtin.INT32:
        return this.write("int32");
      case Builtin.INT64:
        return this.write("int64");
      case Builtin.UINT:
        return this.write("uint");
      case Builtin.UINT8:
        return this.write("uint8");
      case Builtin.UINT16:
        return this.write("uint16");
      case Builtin.UINT32:
        return this.write("uint32");
      case Builtin.UINT64:
        return this.write("uint64");
      case Builtin.FLOAT32:
        return this.write("float32");
      case Builtin.FLOAT64:
        return this.write("float64");
      case Builtin.STRING:
        return this.write("string");
      case Builtin.BYTES:
        return this.write("[]byte");
      case Builtin.TIME:
        return this.write("Time");
      case Builtin.UUID:
        return this.write("UUID");
      case Builtin.JSON:
        return this.write("any");
      case Builtin.USER_ID:
        return this.write("UserID");
      case Builtin.UNRECOGNIZED:
        return this.write("<unknown>");
    }

    return unreachableUnknownType(t);
  }

  protected writeSeenDecl(decl: Decl): void {
    this.write("null");
  }
}

export class JSONDialect extends TextBasedDialect {
  constructor(md: APIMeta) {
    super(md, { name: "javascript", json: true });
  }

  writeSeenDecl(decl: Decl) {
    this.write("null");
  }

  public structBits(
    t: StructType,
    asGoStruct: boolean,
    queryParamsAsObject?: boolean
  ): [string, string, string] {
    const writeObj = (fields: DescribedField[]): string => {
      const oldBuf = this.buf;
      this.buf = [];

      this.level++;
      for (let i = 0; i < fields.length; i++) {
        const f = fields[i];
        this.writeln();
        this.indent();
        this.write(`"${asGoStruct ? f.SrcName : f.name}": `);
        this.writeType(f.typ);
        if (i < fields.length - 1) {
          this.write(",");
        }
      }
      this.level--;
      this.indent();

      const toReturn = this.buf.join("");
      this.buf = oldBuf;
      return toReturn;
    };

    const fields = splitFieldsByLocation(t, this.method, this.asResponse);

    let query = "";
    let headers = "";
    let json = "";

    if (fields[FieldLocation.Query].length > 0) {
      if (asGoStruct || queryParamsAsObject) {
        query = writeObj(fields[FieldLocation.Query]);
      } else {
        const oldBuf = this.buf;
        this.buf = [];

        this.write("?");

        let firstField = true;
        for (const field of fields[FieldLocation.Query]) {
          if (firstField) {
            firstField = false;
          } else {
            this.write("&");
          }

          this.write(field.name, "=");

          if (field.typ.builtin) {
            this.renderBuiltin(field.typ.builtin, false, true);
          } else if (field.typ.list) {
            this.renderBuiltin(field.typ.list.elem.builtin!, false, true);

            // show it's a list by duplicating :-)
            this.write("&", field.name, "=");
            this.renderBuiltin(field.typ.list.elem.builtin!, true, true);
          }
        }

        query = this.buf.join("");
        this.buf = oldBuf;
      }
    }

    if (fields[FieldLocation.Header].length > 0) {
      headers = writeObj(fields[FieldLocation.Header]);
    }

    if (fields[FieldLocation.Body].length > 0) {
      json = writeObj(fields[FieldLocation.Body]);
    }

    return [query, headers, json];
  }

  protected renderStruct(t: StructType, topLevel?: boolean) {
    if (topLevel) {
      const [query, headers, json] = this.structBits(t, false);

      let previousSection = false;
      if (query) {
        this.write("// Query String\n", query);
        previousSection = true;
      }

      if (headers) {
        if (previousSection) {
          this.write("\n\n");
        }

        this.write("// HTTP Headers\n{", headers, "\n}");
        previousSection = true;
      }

      if (json) {
        if (previousSection) {
          this.write("\n\n// JSON Payload\n");
        }

        this.write("{", json, "\n}");
      }

      return;
    }

    this.writeln("{");
    this.level++;
    for (let i = 0; i < t.fields.length; i++) {
      const f = t.fields[i];

      if (f.json_name == "-") {
        continue;
      }

      this.indent();
      this.write(`"${f.json_name !== "" ? f.json_name : f.name}": `);
      this.writeType(f.typ);
      if (i < t.fields.length - 1) {
        this.write(",");
      }
      this.writeln();
    }
    this.level--;
    this.indent();
    this.write("}");

    const toReturn = this.buf.join("");
  }

  protected renderMap(t: MapType) {
    this.writeln("{");
    this.level++;
    this.indent();
    this.writeType(t.key);
    this.write(": ");
    this.writeType(t.value);
    this.writeln();

    this.level--;
    this.indent();
    this.write("}");
  }

  protected renderList(t: ListType) {
    this.write("[");
    this.writeType(t.elem, false, false);
    this.write(", ");
    this.writeType(t.elem, false, true);
    this.write("]");
  }

  protected renderPointer(t: Pointer, topLevel?: boolean) {
    this.writeType(t.base, topLevel);
  }

  protected renderBuiltin(t: Builtin, alt?: boolean, urlEncode?: boolean) {
    let write = (s: string) => {
      if (!urlEncode) {
        return this.write(s);
      }

      if (s[0] === '"' && s[s.length - 1] === '"') {
        s = s.substring(1, s.length - 1);
      }

      return this.write(encodeURIComponent(s));
    };

    switch (t) {
      case Builtin.ANY:
        return write("<any data>");
      case Builtin.BOOL:
        return write(alt ? "false" : "true");
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
        return write(alt ? "2" : "1");
      case Builtin.FLOAT32:
      case Builtin.FLOAT64:
        return write(alt ? "42.9" : "2.3");
      case Builtin.STRING:
        return write(alt ? '"another string"' : '"some string"');
      case Builtin.BYTES:
        return write('"YmFzZTY0Cg=="'); // base64
      case Builtin.TIME:
        return write('"2009-11-10T23:00:00Z"');
      case Builtin.UUID:
        return write('"7d42f515-3517-4e76-be13-30880443546f"');
      case Builtin.JSON:
        return write('{"some json data": true}');
      case Builtin.USER_ID:
        return write('"userID"');
      case Builtin.UNRECOGNIZED:
        return write("<unknown>");
    }

    return unreachableUnknownType(t);
  }
}

class CurlDialect extends TextBasedDialect {
  constructor(meta: APIMeta) {
    super(meta, { name: "javascript", json: true });
  }

  protected renderBuiltin(t: Builtin, altValue?: boolean): void {
    throw new Error("unexpected call");
  }

  protected renderList(t: ListType): void {
    throw new Error("unexpected call");
  }

  protected renderPointer(t: Pointer, topLevel?: boolean) {
    throw new Error("unexpected call");
  }

  protected renderMap(t: MapType): void {
    throw new Error("unexpected call");
  }

  protected renderStruct(t: StructType, topLevel?: boolean): void {
    if (!topLevel) {
      throw new Error("expected top level call only");
    }

    const addr = undefined;
    const path =
      this.rpc?.path.segments
        .map((s) => {
          switch (s.type) {
            case PathSegment_SegmentType.PARAM:
              return ":" + s.value;
            case PathSegment_SegmentType.WILDCARD:
              return "*" + s.value;
            default:
              return s.value;
          }
        })
        .join("/") ?? "";

    if (this.asResponse) {
      const render = new JSONDialect(this.meta);
      render.method = this.method;
      render.asResponse = this.asResponse;
      render.typeArgumentStack = this.typeArgumentStack;
      this.write(render.renderAsText({ struct: t } as Type));
      return;
    }

    const namedTypeToHJSON = (struct: StructType): string => {
      const render = new JSONDialect(this.meta);
      render.method = this.method;
      render.asResponse = false;
      render.typeArgumentStack = this.typeArgumentStack;
      const [queryString, headers, js] = render.structBits(struct, true);

      let bits: string[] = ["{\n"];
      let previousSection = false;
      if (headers) {
        bits.push("    // HTTP headers", headers);
        previousSection = true;
      }
      if (queryString) {
        if (previousSection) {
          bits.push(",\n\n");
        }

        bits.push("    // Query string", queryString);
        previousSection = true;
      }
      if (js) {
        if (previousSection) {
          bits.push(",\n\n");
        }

        bits.push("    // HTTP body", js);
      }
      bits.push("\n}");

      return bits.join("");
    };

    let headers: Record<string, any> = {};
    let queryString = "";

    function addQuery(name: string, value: any) {
      if (Array.isArray(value)) {
        return value.map((v) => {
          addQuery(name, v);
        });
      }

      if (queryString) {
        queryString += "&";
      } else {
        queryString = "?";
      }
      queryString += name + "=" + encodeURIComponent(value);
    }

    const newBody: Record<string, any> = {};
    const processStruct = (struct: StructType, payload: string) => {
      try {
        const astFields = struct.fields;

        const bodyFields: Record<string, any> = HJSON.parse(payload);
        if (typeof bodyFields !== "object") {
          throw new Error("Request Body isn't a JSON object");
        }

        for (const fieldName in bodyFields) {
          if (!bodyFields.hasOwnProperty(fieldName)) {
            continue;
          }

          const fieldValue = bodyFields[fieldName];

          for (const f of astFields) {
            if (f.name === fieldName) {
              let [encodedName, location] = fieldNameAndLocation(f, this.method, false);

              switch (location) {
                case FieldLocation.Header:
                  headers[encodedName] = fieldValue;
                  break;
                case FieldLocation.Query:
                  addQuery(encodedName, fieldValue);
                  break;
                case FieldLocation.Body:
                  newBody[encodedName] = fieldValue;
                  break;
              }
            }
          }
        }
      } catch (e) {
        console.error("Unable to parse body: ", e);
        // but continue anyway
      }
    };

    processStruct(t, namedTypeToHJSON(t));
    if (this.meta.auth_handler?.params?.named) {
      const authStruct = this.meta.decls[this.meta.auth_handler.params.named.id].type.struct!;
      processStruct(authStruct, namedTypeToHJSON(authStruct));
    }

    let reqBody = HJSON.stringify(newBody, {
      quotes: "strings",
      separator: true,
      space: "  ",
    });

    const hasBody = this.rpc ? rpcHasBody(this.meta, this.rpc, this.method) : false;
    const defaultMethod = hasBody ? "POST" : "GET";
    let cmd = "curl ";
    if (this.method !== defaultMethod) {
      cmd += `-X ${this.method} `;
    }
    cmd += `'http://${addr ?? "localhost:4000"}/${path}${queryString}'`;

    for (const header in headers) {
      cmd += ` \\\n  -H '${header}: ${headers[header]}'`;
    }

    if (hasBody) {
      reqBody = reqBody.split("\n").join("\n  ");
      cmd += ` \\\n  -d '${reqBody}'`;
    }

    this.write(cmd);
  }

  protected writeSeenDecl(): void {}
}

class TableDialect extends DialectIface {
  typeArgumentStack: Type[][] = [];

  render(d: Type, service: Service, rpc: RPC, method: string, asResponse: boolean) {
    if (!d?.named) {
      throw new Error("TableDialect can only rendered named structs");
    }

    const st = this.meta.decls[d.named.id].type.struct;
    if (!st) {
      throw new Error("TableDialect can only render named structs");
    }

    this.typeArgumentStack.push(d.named.type_arguments);
    return this.renderStruct(st, 0, method, asResponse);
  }

  renderStruct(t: StructType, level: number, method: string, asResponse: boolean): JSX.Element {
    return (
      <div className={level !== 0 ? "border-gray-200 rounded-sm" : ""}>
        {t.fields.map((f, i) => {
          const [name, location] = fieldNameAndLocation(f, method, asResponse);

          if (location === FieldLocation.UnusedField) {
            return <Fragment key={f.name} />;
          }

          return (
            <div key={f.name} className={i > 0 ? "border-gray-200 mt-1 border-t pt-1" : ""}>
              <div className="flex font-mono leading-6">
                <div className="text-gray-900 text-sm font-bold">{f.name}</div>
                <div className="text-gray-500 ml-2 flex-grow p-0.5 text-xs">
                  {this.describeType(f.typ)}
                </div>
                <div
                  className="text-gray-700 bg-gray-100 cursor-help rounded p-1 text-center text-xs"
                  title={locationDescription(name, location)}
                >
                  {location}
                </div>
              </div>
              <div className="flex">
                {f.doc !== "" ? (
                  <div className="text-gray-700 flex-grow text-sm">{f.doc}</div>
                ) : (
                  <div className="text-gray-400 flex-grow text-xs">No description.</div>
                )}
                <div
                  className="text-gray-500 font-mono text-xs"
                  title={"The encoded field name on the wire"}
                >
                  {name}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    );
  }

  describeType(t: Type): string {
    return t.struct
      ? "struct"
      : t.map
      ? "map"
      : t.list
      ? "list of " + this.describeType(t.list.elem)
      : t.builtin
      ? this.describeBuiltin(t.builtin)
      : t.named
      ? this.describeNamed(t.named)
      : t.type_parameter
      ? this.describeTypeParameter(t.type_parameter)
      : t.pointer
      ? this.describePointer(t.pointer)
      : "<unknown>";
  }

  describeTypeParameter(t: TypeParameterRef): string {
    const typeArgument = this.typeArgumentStack[this.typeArgumentStack.length - 1][t.param_idx];
    return this.describeType(typeArgument);
  }

  describeBuiltin(t: Builtin): string {
    switch (t) {
      case Builtin.ANY:
        return "<any>";
      case Builtin.BOOL:
        return "boolean";
      case Builtin.INT:
        return "int";
      case Builtin.INT8:
        return "int";
      case Builtin.INT16:
        return "int";
      case Builtin.INT32:
        return "int";
      case Builtin.INT64:
        return "int";
      case Builtin.UINT:
        return "uint";
      case Builtin.UINT8:
        return "uint";
      case Builtin.UINT16:
        return "uint";
      case Builtin.UINT32:
        return "uint";
      case Builtin.UINT64:
        return "uint";
      case Builtin.FLOAT32:
        return "float";
      case Builtin.FLOAT64:
        return "float";
      case Builtin.STRING:
        return "string";
      case Builtin.BYTES:
        return "bytes";
      case Builtin.TIME:
        return "RFC 3339-formatted timestamp";
      case Builtin.UUID:
        return "UUID";
      case Builtin.JSON:
        return "arbitrary JSON";
      case Builtin.USER_ID:
        return "User ID";
      case Builtin.UNRECOGNIZED:
        return "<unknown>";
    }

    return unreachableUnknownType(t);
  }

  describeNamed(named: NamedType): string {
    const decl = this.meta.decls[named.id];

    let types = "";
    if (named.type_arguments.length > 0) {
      types = "[";

      for (let i = 0; i < named.type_arguments.length; i++) {
        if (i > 0) {
          types += ", ";
        }

        types += this.describeType(named.type_arguments[i]);
      }

      types += "]";
    }

    return decl.loc.pkg_name + "." + decl.name + types;
  }

  describePointer(ptr: Pointer): string {
    return this.describeType(ptr.base);
  }
}

const dialects: { [key in Dialect]: (meta: APIMeta) => DialectIface } = {
  go: (meta) => new GoDialect(meta),
  typescript: (meta) => new TypescriptDialect(meta),
  json: (meta) => new JSONDialect(meta),
  curl: (meta) => new CurlDialect(meta),
  table: (meta) => new TableDialect(meta),
};

// This function serves two purposes
//
// 1. If we ever hit it at runtime; we'll return "<unknown>" to be rendered
// 2. If we have a switch statement on an enum without a default, and return from each case then if we miss one of the
//    enum options, we'll get a compile error if we try and pass that case as the parameter x to this function
export function unreachableUnknownType(_: never): string {
  return "<unknown>";
}
