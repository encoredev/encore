import {Field} from "../../../../../../../proto/encore/parser/schema/v1/schema.pb"
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

export enum FieldLocation {
    Body = "JSON Payload",
    Header = "HTTP Header",
    Query = "Query String",
}

export function fieldNameAndLocation(f: Field, method: string, asResponse: boolean): [string, FieldLocation] {
    for (const tag of f.tags) {
        switch (tag.key) {
            case "qs":
            case "query":
                if (!asResponse) {
                    return [tag.name, FieldLocation.Query]
                }
                break
            case "header":
                return [tag.name, FieldLocation.Header]
            case "json":
                return [tag.name, FieldLocation.Body]
        }
    }

    return [f.name, FieldLocation.Body]
}

export function locationDescription(name: string, location: FieldLocation): string {
    switch (location) {
        case FieldLocation.Header:
            return `"${name}" is sent as a HTTP Header`
        case FieldLocation.Query:
            return `"${name}" is sent as a query string parameter`
        default:
            return ""
    }
}

export function splitFieldsByLocation(t: StructType, method: string, asResponse: boolean): Record<FieldLocation, Field[]> {
    let rtn: Record<FieldLocation, Field[]> = {
        [FieldLocation.Body]: [],
        [FieldLocation.Query]: [],
        [FieldLocation.Header]: []
    }

    for (const field of t.fields) {
        const [name, location] = fieldNameAndLocation(field, method, asResponse)

        const newField = {...field, name}

        rtn[location].push(newField)
    }

    return rtn
}
