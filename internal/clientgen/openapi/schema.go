package openapi

import (
	"fmt"
	"math"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/getkin/kin-openapi/openapi3"

	"encr.dev/parser/encoding"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

func (g *Generator) bodyContent(params []*encoding.ParameterEncoding) openapi3.Content {
	if len(params) == 0 {
		return nil
	}

	required := make([]string, 0, len(params))
	props := make(openapi3.Schemas)
	for _, p := range params {
		val := g.schemaType(p.Type)
		if vv := val.Value; vv != nil {
			vv.Title, vv.Description = splitDoc(p.Doc)
		}
		props[p.WireFormat] = val
		if !p.Optional {
			required = append(required, p.WireFormat)
		}
	}

	s := openapi3.NewObjectSchema()
	s.Properties = props
	s.Required = required

	return openapi3.Content{
		"application/json": &openapi3.MediaType{
			Schema:   s.NewRef(),
			Example:  nil,
			Examples: nil,
			Encoding: nil,
		},
	}
}

func (g *Generator) schemaType(typ *schema.Type) *openapi3.SchemaRef {
	switch t := typ.Typ.(type) {
	// A type switch for all the different schema types we support
	case *schema.Type_Named:
		return g.namedSchemaType(t.Named)

	case *schema.Type_Struct:
		props := make(openapi3.Schemas)
		required := make([]string, 0, len(t.Struct.Fields))
		for _, f := range t.Struct.Fields {
			jsonName := f.JsonName
			if jsonName == "-" {
				continue
			}
			if jsonName == "" {
				jsonName = f.Name
			}
			if !f.Optional {
				required = append(required, jsonName)
			}

			val := g.schemaType(f.Typ)

			if vv := val.Value; vv != nil {
				vv.Title, vv.Description = splitDoc(f.Doc)
			}
			props[jsonName] = val
		}

		s := openapi3.NewObjectSchema()
		s.Properties = props
		s.Required = required
		return s.NewRef()

	case *schema.Type_Map:
		// TODO non-string keys are not supported
		s := openapi3.NewObjectSchema()
		s.AdditionalProperties = openapi3.AdditionalProperties{
			Schema: g.schemaType(t.Map.Value),
		}
		return s.NewRef()

	case *schema.Type_List:
		arr := openapi3.NewArraySchema()
		arr.Items = g.schemaType(t.List.Elem)
		return arr.NewRef()

	case *schema.Type_Pointer:
		return g.schemaType(t.Pointer.Base)

	case *schema.Type_Literal:
		switch tt := t.Literal.Value.(type) {
		case *schema.Literal_Str:
			return openapi3.NewStringSchema().WithEnum(tt.Str).NewRef()
		case *schema.Literal_Boolean:
			return openapi3.NewBoolSchema().WithEnum(tt.Boolean).NewRef()
		case *schema.Literal_Int:
			return openapi3.NewInt64Schema().WithEnum(tt.Int).NewRef()
		case *schema.Literal_Float:
			return openapi3.NewFloat64Schema().WithEnum(tt.Float).NewRef()
		case *schema.Literal_Null:
			// This shouldn't happen in most situations as we handle literals explicitly.
			return openapi3.NewBoolSchema().WithNullable().NewRef()
		default:
			doBailout(errors.Newf("unknown literal type %T", tt))
			return nil // unreachable
		}

	case *schema.Type_Union:
		// First check if all the fields are literals.
		// If so we can more accurately represent this union as an enum.
		var (
			literals        []any
			literalsType    string
			haveAllLiterals = true
			haveLiteralNull bool
		)
		for _, tt := range t.Union.Types {
			lit, ok := tt.Typ.(*schema.Type_Literal)
			if !ok {
				// It's not a literal.
				// Still need to keep going to find any nulls.
				haveAllLiterals = false
				continue
			}

			var litType string
			switch tt := lit.Literal.Value.(type) {
			case *schema.Literal_Str:
				litType = openapi3.TypeString
				literals = append(literals, tt.Str)
			case *schema.Literal_Boolean:
				litType = openapi3.TypeBoolean
				literals = append(literals, tt.Boolean)
			case *schema.Literal_Int:
				litType = openapi3.TypeInteger
				literals = append(literals, tt.Int)
			case *schema.Literal_Float:
				litType = openapi3.TypeNumber
				literals = append(literals, tt.Float)
			case *schema.Literal_Null:
				haveLiteralNull = true
				continue
			default:
				doBailout(errors.Newf("unknown literal type %T", tt))
			}

			// Set the literals type if we haven't seen it yet.
			if literalsType == "" {
				literalsType = litType
			} else if literalsType != litType {
				// If we have different types, it can't be represented as an enum.
				haveAllLiterals = false
			}
		}

		if haveAllLiterals {
			s := openapi3.NewSchema()
			s.Type = literalsType
			s.Nullable = haveLiteralNull
			return s.WithEnum(literals...).NewRef()
		}

		// Otherwise, we have to represent this as an anyOf schema.
		schemas := make([]*openapi3.Schema, 0, len(t.Union.Types))
		for _, tt := range t.Union.Types {
			schemas = append(schemas, g.schemaType(tt).Value)
		}

		s := openapi3.NewAnyOfSchema(schemas...)
		s.Nullable = haveLiteralNull
		return s.NewRef()

	case *schema.Type_TypeParameter:
		return openapi3.NewObjectSchema().NewRef() // unknown

	case *schema.Type_Config:
		elem := g.schemaType(t.Config.Elem)
		if t.Config.IsValuesList {
			s := openapi3.NewArraySchema()
			s.Items = elem
			return s.NewRef()
		} else {
			return elem
		}

	case *schema.Type_Builtin:
		return g.builtinSchemaType(t.Builtin).NewRef()

	default:
		doBailout(errors.Newf("unknown schema type %T", t))
		panic("unreachable")
	}
}

func (g *Generator) builtinSchemaType(t schema.Builtin) *openapi3.Schema {
	switch t {
	case schema.Builtin_BOOL:
		return openapi3.NewBoolSchema()
	case schema.Builtin_INT8:
		return openapi3.NewInt32Schema().WithMin(math.MinInt8).WithMax(math.MaxInt8)
	case schema.Builtin_INT16:
		return openapi3.NewInt32Schema().WithMin(math.MinInt16).WithMax(math.MaxInt16)
	case schema.Builtin_INT32:
		return openapi3.NewInt32Schema().WithMin(math.MinInt32).WithMax(math.MaxInt32)
	case schema.Builtin_INT64, schema.Builtin_INT:
		return openapi3.NewInt64Schema()
	case schema.Builtin_UINT8:
		return openapi3.NewInt32Schema().WithMin(0).WithMax(math.MaxUint8)
	case schema.Builtin_UINT16:
		return openapi3.NewInt32Schema().WithMin(0).WithMax(math.MaxUint16)
	case schema.Builtin_UINT32:
		return openapi3.NewInt64Schema().WithMin(0).WithMax(math.MaxUint32)
	case schema.Builtin_UINT64, schema.Builtin_UINT:
		return openapi3.NewInt64Schema().WithMin(0)
	case schema.Builtin_FLOAT32, schema.Builtin_FLOAT64:
		return openapi3.NewFloat64Schema()
	case schema.Builtin_STRING:
		return openapi3.NewStringSchema()
	case schema.Builtin_BYTES:
		return openapi3.NewStringSchema().WithFormat("byte")
	case schema.Builtin_TIME:
		return openapi3.NewStringSchema().WithFormat("date-time")
	case schema.Builtin_UUID:
		return openapi3.NewUUIDSchema()
	case schema.Builtin_JSON:
		return openapi3.NewObjectSchema()
	case schema.Builtin_USER_ID:
		return openapi3.NewStringSchema()
	default:
		doBailout(errors.Newf("unknown builtin type %v", t))
		panic("unreachable")
	}
}

func (g *Generator) namedSchemaType(typ *schema.Named) *openapi3.SchemaRef {
	namedType := &schema.Type{Typ: &schema.Type_Named{Named: typ}}
	concrete, err := encoding.GetConcreteType(g.md.Decls, namedType, nil)
	if err != nil {
		doBailout(errors.Wrap(err, "get concrete type"))
	}

	origCandidate := g.typeToDefinitionName(namedType)

	// Make sure the candidate name corresponds to this declaration.
	for idx := 1; ; idx++ {
		candidate := origCandidate
		// Add a suffix if this is not the first candidate.
		if idx > 1 {
			candidate += fmt.Sprintf("_%d", idx)
		}

		if _, ok := g.spec.Components.Schemas[candidate]; ok {
			// There is already a declaration with that name; make sure it matches
			if seen, ok := g.seenDecls[candidate]; ok && seen != typ.Id {
				// Different declaration; try again.
				continue
			}
		} else {
			// Unused name; allocate it.
			// Write to the maps before we compute the schema to avoid infinite recursion
			// in the presence of recursive types.
			g.seenDecls[candidate] = typ.Id
			g.spec.Components.Schemas[candidate] = nil

			g.spec.Components.Schemas[candidate] = g.schemaType(concrete)
		}

		return &openapi3.SchemaRef{
			Ref: "#/components/schemas/" + candidate,
		}
	}
}

func (g *Generator) typeToDefinitionName(typ *schema.Type) string {
	switch typ := typ.Typ.(type) {
	case *schema.Type_Named:
		var name strings.Builder
		decl := g.md.Decls[typ.Named.Id]
		name.WriteString(decl.Loc.PkgName)
		name.WriteString(".")
		name.WriteString(decl.Name)
		for _, typeArg := range typ.Named.TypeArguments {
			name.WriteString("_")
			name.WriteString(g.typeToDefinitionName(typeArg))
		}
		return name.String()
	case *schema.Type_List:
		return "List_" + g.typeToDefinitionName(typ.List.Elem)
	case *schema.Type_Map:
		return "Map_" + g.typeToDefinitionName(typ.Map.Key) + "_" + g.typeToDefinitionName(typ.Map.Value)
	case *schema.Type_Pointer:
		return g.typeToDefinitionName(typ.Pointer.Base)
	case *schema.Type_Config:
		return g.typeToDefinitionName(typ.Config.Elem)
	case *schema.Type_Builtin:
		switch typ.Builtin {
		case schema.Builtin_ANY:
			return "any"
		case schema.Builtin_BOOL:
			return "bool"
		case schema.Builtin_INT8:
			return "int8"
		case schema.Builtin_INT16:
			return "int16"
		case schema.Builtin_INT32:
			return "int32"
		case schema.Builtin_INT64:
			return "int64"
		case schema.Builtin_UINT8:
			return "uint8"
		case schema.Builtin_UINT16:
			return "uint16"
		case schema.Builtin_UINT32:
			return "uint32"
		case schema.Builtin_UINT64:
			return "uint64"
		case schema.Builtin_FLOAT32:
			return "float32"
		case schema.Builtin_FLOAT64:
			return "float64"
		case schema.Builtin_STRING:
			return "string"
		case schema.Builtin_BYTES:
			return "bytes"
		case schema.Builtin_TIME:
			return "string"
		case schema.Builtin_UUID:
			return "string"
		case schema.Builtin_JSON:
			return "string"
		case schema.Builtin_USER_ID:
			return "string"
		case schema.Builtin_INT:
			return "int"
		case schema.Builtin_UINT:
			return "uint"
		default:
			return ""
		}
	}

	return ""
}

func (g *Generator) pathParamType(typ meta.PathSegment_ParamType) *openapi3.Schema {
	switch typ {
	case meta.PathSegment_BOOL:
		return openapi3.NewBoolSchema()
	case meta.PathSegment_INT8:
		return openapi3.NewInt32Schema().WithMin(math.MinInt8).WithMax(math.MaxInt8)
	case meta.PathSegment_INT16:
		return openapi3.NewInt32Schema().WithMin(math.MinInt16).WithMax(math.MaxInt16)
	case meta.PathSegment_INT32:
		return openapi3.NewInt32Schema().WithMin(math.MinInt32).WithMax(math.MaxInt32)
	case meta.PathSegment_INT64, meta.PathSegment_INT:
		return openapi3.NewInt64Schema()
	case meta.PathSegment_UINT8:
		return openapi3.NewInt32Schema().WithMin(0).WithMax(math.MaxUint8)
	case meta.PathSegment_UINT16:
		return openapi3.NewInt32Schema().WithMin(0).WithMax(math.MaxUint16)
	case meta.PathSegment_UINT32:
		return openapi3.NewInt64Schema().WithMin(0).WithMax(math.MaxUint32)
	case meta.PathSegment_UINT64, meta.PathSegment_UINT:
		return openapi3.NewInt64Schema().WithMin(0)
	case meta.PathSegment_STRING:
		return openapi3.NewStringSchema()
	case meta.PathSegment_UUID:
		return openapi3.NewUUIDSchema()
	default:
		doBailout(errors.Newf("unknown path param type: %v"))
		panic("unreachable")
	}
}
