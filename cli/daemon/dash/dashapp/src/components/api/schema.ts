import {Field} from "../../../../../../../proto/encore/parser/schema/v1/schema.pb"
import * as pb from '../../../../../../../proto/encore/parser/schema/v1/schema.pb'
import { APIMeta, RPC } from "./api"
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
    UnusedField = "hidden",
}

export interface DescribedField extends Field {
    SrcName: string
}

export function methodSupportsPayloads(method: string): boolean {
    return method !== "GET" && method !== "HEAD" && method !== "DELETE"
}

export function rpcHasBody(md: APIMeta, rpc: RPC, method: string) {
    const named = rpc.request_schema?.named
    if (!named) {
        return false
    }
    const astFields = md.decls[named.id].type.struct!.fields
    for (const f of astFields) {
        let [_, location] = fieldNameAndLocation(f, method, false)
        if (location === FieldLocation.Body) {
            return true
        }
    }
    return false
}

export function fieldNameAndLocation(f: Field, method: string, asResponse: boolean): [string, FieldLocation] {
    for (const tag of f.tags) {
        switch (tag.key) {
            case "qs":
            case "query":
                if (tag.name === "-") {
                    return ["-", FieldLocation.UnusedField]
                }

                if (!asResponse) {
                    return [tag.name, FieldLocation.Query]
                }
                break
            
            case "header":
                if (tag.name === "-") {
                    return ["-", FieldLocation.UnusedField]
                }

                return [tag.name, FieldLocation.Header]

            case "json":
                if (tag.name === "-") {
                    return ["-", FieldLocation.UnusedField]
                }

                if (methodSupportsPayloads(method) || asResponse) {
                    return [tag.name, FieldLocation.Body]
                }
        }
    }

    if (methodSupportsPayloads(method) || asResponse) {
        return [f.name, FieldLocation.Body]
    } else {
        return [f.name, FieldLocation.Query]
    }
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

export function splitFieldsByLocation(t: StructType, method: string, asResponse: boolean): Record<FieldLocation, DescribedField[]> {
    let rtn: Record<FieldLocation, DescribedField[]> = {
        [FieldLocation.Body]: [],
        [FieldLocation.Query]: [],
        [FieldLocation.Header]: [],
        [FieldLocation.UnusedField]: [],
    }

    for (const field of t.fields) {
        const [name, location] = fieldNameAndLocation(field, method, asResponse)

        const newField = {...field, SrcName: field.name, name: name}

        rtn[location].push(newField)
    }

    return rtn
}
