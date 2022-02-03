import * as pb from '../../../../../../../proto/encore/parser/schema/v1/schema.pb'
export * from '../../../../../../../proto/encore/parser/schema/v1/schema.pb'

// Aliases to match old type names
export type ListType = pb.List
export type MapType = pb.Map
export type NamedType = pb.Named
export type StructType = pb.Struct

// export enums
export {
    Builtin as BuiltinType,
}  from '../../../../../../../proto/encore/parser/schema/v1/schema.pb'
