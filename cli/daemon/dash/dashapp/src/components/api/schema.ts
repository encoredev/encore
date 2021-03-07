export type TypeName = "struct" | "map" | "list" | "builtin" | "named";

export const typeName: (typ: Type) => TypeName = (typ: Type) => (
  typ.struct ? "struct" :
  typ.map ? "map" :
  typ.list ? "list" :
  typ.named ? "named" :
  "builtin"
)

export interface Type {
  // oneof these
  struct?: StructType;
  map?: MapType;
  list?: ListType;
  builtin?: BuiltinType;
  named?: NamedType;
}

export interface StructType {
  fields: Field[];
}

export interface Field {
  typ: Type;
  name: string;
  doc: string;
  json_name: string;
  optional: boolean;
}

export interface MapType {
  key: Type;
  value: Type;
}

export interface ListType {
  elem: Type;
}

export interface NamedType {
  id: number;
}

export interface Decl {
  id: number;
  name: string;
  type: Type;
  doc: string;
  loc: Loc;
}

export interface Loc {
  pkg_path: string;
  pkg_name: string;
  filename: string;
  start_pos: number;
  end_pos: number;
  src_line_start: number;
  src_line_end: number;
  src_col_start: number;
  src_col_end: number;
}

export enum BuiltinType {
  Any = "ANY",
  Bool = "BOOL",
  Int8 = "INT8",
  Int16 = "INT16",
  Int32 = "INT32",
  Int64 = "INT64",
  Uint8  = "UINT8",
  Uint16 = "UINT16",
  Uint32 = "UINT32",
  Uint64 = "UINT64",
  Float32 = "FLOAT32",
  Float64 = "FLOAT64",
  String = "STRING",
  Bytes = "BYTES",
  Time = "TIME",
  UUID = "UUID",
  JSON = "JSON",
  USER_ID = "USER_ID",
}