/* eslint-disable */
export const protobufPackage = "encore.parser.schema.v1";

/**
 * Builtin represents a type which Encore (and Go) have inbuilt support for and so can be represented by Encore's tooling
 * directly, rather than needing to understand the full implementation details of how the type is structured.
 */
export enum Builtin {
  /** ANY - Inbuilt Go Types */
  ANY = "ANY",
  BOOL = "BOOL",
  INT8 = "INT8",
  INT16 = "INT16",
  INT32 = "INT32",
  INT64 = "INT64",
  UINT8 = "UINT8",
  UINT16 = "UINT16",
  UINT32 = "UINT32",
  UINT64 = "UINT64",
  FLOAT32 = "FLOAT32",
  FLOAT64 = "FLOAT64",
  STRING = "STRING",
  BYTES = "BYTES",
  /** TIME - Additional Encore Types */
  TIME = "TIME",
  UUID = "UUID",
  JSON = "JSON",
  USER_ID = "USER_ID",
  INT = "INT",
  UINT = "UINT",
  UNRECOGNIZED = "UNRECOGNIZED",
}

/**
 * Type represents the base of our schema on which everything else is built on-top of. It has to be one, and only one,
 * thing from our list of meta types.
 *
 * A type may be concrete or abstract, however to determine if a type is abstract you need to recursive through the
 * structures looking for any uses of the TypeParameterPtr type
 */
export interface Type {
  /** Concrete / non-parameterized Types */
  named: Named | undefined;
  /** The type is a struct definition */
  struct: Struct | undefined;
  /** The type is a map */
  map: Map | undefined;
  /** The type is a slice */
  list: List | undefined;
  /** The type is one of the base built in types within Go */
  builtin: Builtin | undefined;
  /** Abstract Types */
  type_parameter: TypeParameterRef | undefined;
  /** Encore Special Types */
  config: ConfigValue | undefined;
}

/** TypeParameterRef is a reference to a `TypeParameter` within a declaration block */
export interface TypeParameterRef {
  /** The ID of the declaration block */
  decl_id: number;
  /** The index of the type parameter within the declaration block */
  param_idx: number;
}

/**
 * Decl represents the declaration of a type within the Go code which is either concrete or _parameterized_. The type is
 * concrete when there are zero type parameters assigned.
 *
 * For example the Go Code:
 * ```
 * // Set[A] represents our set type
 * type Set[A any] = map[A]struct{}
 * ```
 *
 * Would become:
 * ```
 * _ = &Decl{
 *     id: 1,
 *     name: "Set",
 *     type: &Type{
 *         typ_map: &Map{
 *             key: &Type { typ_type_parameter: ... reference to "A" type parameter below ... },
 *             value: &Type { typ_struct: ... empty struct type ... },
 *         },
 *     },
 *     typeParameters: []*TypeParameter{ { name: "A" } },
 *     doc: "Set[A] represents our set type",
 *     loc: &Loc { ... },
 * }
 * ```
 */
export interface Decl {
  /** A internal ID which we can refer to this declaration by */
  id: number;
  /** The name of the type as assigned in the code */
  name: string;
  /** The underlying type of this declaration */
  type: Type;
  /** Any type parameters on this declaration (note; instantiated types used within this declaration would not be captured here) */
  type_params: TypeParameter[];
  /** The comment block on the type */
  doc: string;
  /** The location of the declaration within the project */
  loc: Loc;
}

/**
 * TypeParameter acts as a place holder for an (as of yet) unknown type in the declaration; the type parameter is
 * replaced with a type argument upon instantiation of the parameterized function or type.
 */
export interface TypeParameter {
  /** The identifier given to the type parameter */
  name: string;
}

/** Loc is the location of a declaration within the code base */
export interface Loc {
  /** The package path within the repo (i.e. `users/signup`) */
  pkg_path: string;
  /** The package name (i.e. `signup`) */
  pkg_name: string;
  /** The file name (i.e. `signup.go`) */
  filename: string;
  /** The starting index within the file for this node */
  start_pos: number;
  /** The ending index within the file for this node */
  end_pos: number;
  /** The starting line within the file for this node */
  src_line_start: number;
  /** The ending line within the file for this node */
  src_line_end: number;
  /** The starting column on the starting line for this node */
  src_col_start: number;
  /** The ending column on the ending line for this node */
  src_col_end: number;
}

/** Named references declaration block by name */
export interface Named {
  /** The `Decl.id` this name refers to */
  id: number;
  /** The type arguments used to instantiate this parameterised declaration */
  type_arguments: Type[];
}

/** Struct contains a list of fields which make up the struct */
export interface Struct {
  fields: Field[];
}

/** Field represents a field within a struct */
export interface Field {
  /** The type of the field */
  typ: Type;
  /** The name of the field */
  name: string;
  /** The comment for the field */
  doc: string;
  /** The optional json name if it's different from the field name. (The value "-" indicates to omit the field.) */
  json_name: string;
  /** Whether the field is optional. */
  optional: boolean;
  /** The query string name to use in GET/HEAD/DELETE requests. (The value "-" indicates to omit the field.) */
  query_string_name: string;
  /** The original Go struct tag; should not be parsed individually */
  raw_tag: string;
  /** Parsed go struct tags. Used for marshalling hints */
  tags: Tag[];
}

export interface Tag {
  /** The tag key (e.g. json, query, header ...) */
  key: string;
  /** The tag name (e.g. first_name, firstName, ...) */
  name: string;
  /** Key Options (e.g. omitempty, optional ...) */
  options: string[];
}

/** Map represents a map Type */
export interface Map {
  /** The type of the key for this map */
  key: Type;
  /** The type of the value of this map */
  value: Type;
}

/** List represents a list type (array or slice) */
export interface List {
  /** The type of the elements in the list */
  elem: Type;
}

/** ConfigValue represents a config value wrapper. */
export interface ConfigValue {
  /** The type of the config value */
  elem: Type;
}
