package mcp

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	schema "encr.dev/proto/encore/parser/schema/v1"
)

// FieldLocation represents where a field is located in the API request/response
type FieldLocation int

const (
	FieldLocationBody   FieldLocation = 0
	FieldLocationQuery  FieldLocation = 1
	FieldLocationHeader FieldLocation = 2
	FieldLocationCookie FieldLocation = 3
	FieldLocationUnused FieldLocation = 4
)

// DescribedField is a field with additional metadata
type DescribedField struct {
	*schema.Field
	SrcName  string
	Name     string
	Location FieldLocation
}

// StructBits generates JSON representations of a struct's fields separated by location
// It returns query, headers, cookies, and JSON body as strings
func StructBits(s *schema.Struct, method string, asResponse bool, asGoStruct bool, queryParamsAsObject bool) (query, headers, cookies, jsonBody string) {
	// Split the fields by location
	fieldsByLocation := splitFieldsByLocation(s, method, asResponse)

	// Generate query string
	if len(fieldsByLocation[FieldLocationQuery]) > 0 {
		if asGoStruct || queryParamsAsObject {
			query = writeFieldsAsJSON(fieldsByLocation[FieldLocationQuery], asGoStruct)
		} else {
			var queryParams []string
			for _, field := range fieldsByLocation[FieldLocationQuery] {
				fieldName := field.Name
				fieldValue := renderFieldValueAsQueryParam(field.Typ)

				queryParams = append(queryParams, url.QueryEscape(fieldName)+"="+fieldValue)

				// If it's a list, add a second parameter to show it's a list
				if field.Typ.GetList() != nil {
					queryParams = append(queryParams, url.QueryEscape(fieldName)+"="+fieldValue)
				}
			}
			query = "?" + strings.Join(queryParams, "&")
		}
	}

	// Generate headers
	if len(fieldsByLocation[FieldLocationHeader]) > 0 {
		headers = writeFieldsAsJSON(fieldsByLocation[FieldLocationHeader], asGoStruct)
	}

	// Generate cookies
	if len(fieldsByLocation[FieldLocationCookie]) > 0 {
		cookies = writeCookiesAsJSON(fieldsByLocation[FieldLocationCookie], asGoStruct)
	}

	// Generate JSON body
	if len(fieldsByLocation[FieldLocationBody]) > 0 {
		jsonBody = writeFieldsAsJSON(fieldsByLocation[FieldLocationBody], asGoStruct)
	}

	return
}

// writeFieldsAsJSON renders a list of fields as a JSON object
func writeFieldsAsJSON(fields []DescribedField, asGoStruct bool) string {
	var buf bytes.Buffer
	buf.WriteString("\n")

	for i, f := range fields {
		fieldName := f.SrcName
		if !asGoStruct {
			fieldName = f.Name
		}

		buf.WriteString("    \"")
		buf.WriteString(fieldName)
		buf.WriteString("\": ")

		renderTypeValue(&buf, f.Typ)

		if i < len(fields)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

// writeCookiesAsJSON renders cookie fields as JSON
func writeCookiesAsJSON(fields []DescribedField, asGoStruct bool) string {
	var buf bytes.Buffer
	buf.WriteString("\n")

	for i, f := range fields {
		fieldName := f.SrcName
		if !asGoStruct {
			fieldName = f.Name
		}

		buf.WriteString("    \"")
		buf.WriteString(fieldName)
		buf.WriteString("\": ")

		// If it's a builtin, render it normally, otherwise render as an empty string
		if f.Typ.GetBuiltin() != schema.Builtin_ANY {
			renderTypeValue(&buf, f.Typ)
		} else {
			buf.WriteString("\"\"")
		}

		if i < len(fields)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

// renderTypeValue renders a type value to the buffer
func renderTypeValue(buf *bytes.Buffer, typ *schema.Type) {
	switch {
	case typ.GetBuiltin() != schema.Builtin_ANY:
		renderBuiltinValue(buf, typ.GetBuiltin(), false)
	case typ.GetList() != nil:
		buf.WriteString("[")
		renderTypeValue(buf, typ.GetList().Elem)
		buf.WriteString("]")
	case typ.GetStruct() != nil:
		buf.WriteString("{")
		for i, f := range typ.GetStruct().Fields {
			if f.JsonName == "-" {
				continue
			}

			jsonName := f.JsonName
			if jsonName == "" {
				jsonName = f.Name
			}

			buf.WriteString("\"")
			buf.WriteString(jsonName)
			buf.WriteString("\": ")

			renderTypeValue(buf, f.Typ)

			if i < len(typ.GetStruct().Fields)-1 {
				buf.WriteString(", ")
			}
		}
		buf.WriteString("}")
	case typ.GetMap() != nil:
		buf.WriteString("{")
		renderTypeValue(buf, typ.GetMap().Key)
		buf.WriteString(": ")
		renderTypeValue(buf, typ.GetMap().Value)
		buf.WriteString("}")
	case typ.GetNamed() != nil:
		// Just render as null for simplicity
		buf.WriteString("null")
	case typ.GetPointer() != nil:
		renderTypeValue(buf, typ.GetPointer().Base)
	case typ.GetUnion() != nil && len(typ.GetUnion().Types) > 0:
		// Just render the first type of the union
		renderTypeValue(buf, typ.GetUnion().Types[0])
	case typ.GetLiteral() != nil:
		renderLiteralValue(buf, typ.GetLiteral())
	default:
		buf.WriteString("<unknown>")
	}
}

// renderBuiltinValue renders a builtin type value
func renderBuiltinValue(buf *bytes.Buffer, b schema.Builtin, urlEncode bool) {
	var value string

	switch b {
	case schema.Builtin_ANY:
		value = "<any data>"
	case schema.Builtin_BOOL:
		value = "false"
	case schema.Builtin_INT, schema.Builtin_INT8, schema.Builtin_INT16, schema.Builtin_INT32, schema.Builtin_INT64,
		schema.Builtin_UINT, schema.Builtin_UINT8, schema.Builtin_UINT16, schema.Builtin_UINT32, schema.Builtin_UINT64:
		value = "0"
	case schema.Builtin_FLOAT32, schema.Builtin_FLOAT64:
		value = "0.0"
	case schema.Builtin_STRING:
		value = "\"\""
	case schema.Builtin_BYTES:
		value = "\"\" /* base64 */"
	case schema.Builtin_TIME:
		value = "\"2009-11-10T23:00:00Z\""
	case schema.Builtin_UUID:
		value = "\"7d42f515-3517-4e76-be13-30880443546f\""
	case schema.Builtin_JSON:
		value = "{}"
	case schema.Builtin_USER_ID:
		value = "\"userID\""
	default:
		value = "<unknown>"
	}

	if urlEncode {
		// Remove quotes for URL encoding if they exist
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		buf.WriteString(url.QueryEscape(value))
	} else {
		buf.WriteString(value)
	}
}

// renderLiteralValue renders a literal value
func renderLiteralValue(buf *bytes.Buffer, lit *schema.Literal) {
	switch v := lit.Value.(type) {
	case *schema.Literal_Boolean:
		if v.Boolean {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case *schema.Literal_Int:
		buf.WriteString(strconv.FormatInt(v.Int, 10))
	case *schema.Literal_Float:
		buf.WriteString(strconv.FormatFloat(v.Float, 'f', -1, 64))
	case *schema.Literal_Str:
		jsonStr, _ := json.Marshal(v.Str)
		buf.Write(jsonStr)
	case *schema.Literal_Null:
		buf.WriteString("null")
	default:
		buf.WriteString("<unknown>")
	}
}

// renderFieldValueAsQueryParam returns a URL-encoded string representation of a field's value
func renderFieldValueAsQueryParam(typ *schema.Type) string {
	var buf bytes.Buffer

	if typ.GetBuiltin() != schema.Builtin_ANY {
		renderBuiltinValue(&buf, typ.GetBuiltin(), true)
	} else if typ.GetList() != nil {
		renderTypeValue(&buf, typ.GetList().Elem)
	} else {
		buf.WriteString("<value>")
	}

	return buf.String()
}

// splitFieldsByLocation categorizes struct fields by their HTTP location
func splitFieldsByLocation(s *schema.Struct, method string, asResponse bool) map[FieldLocation][]DescribedField {
	result := make(map[FieldLocation][]DescribedField)

	for _, f := range s.Fields {
		name, location := fieldNameAndLocation(f, method, asResponse)

		// Skip unused fields
		if location == FieldLocationUnused {
			continue
		}

		result[location] = append(result[location], DescribedField{
			Field:    f,
			SrcName:  f.Name,
			Name:     name,
			Location: location,
		})
	}

	return result
}

// fieldNameAndLocation determines the name and location of a field based on HTTP method and tags
func fieldNameAndLocation(f *schema.Field, method string, asResponse bool) (string, FieldLocation) {
	// For response, all fields go in the body unless explicitly tagged
	if asResponse {
		// Check for explicit wire location
		if f.Wire != nil {
			if f.Wire.GetHeader() != nil {
				name := f.Wire.GetHeader().GetName()
				if name == "" {
					name = f.Name
				}
				return name, FieldLocationHeader
			} else if f.Wire.GetQuery() != nil {
				name := f.Wire.GetQuery().GetName()
				if name == "" {
					name = f.Name
				}
				return name, FieldLocationQuery
			}
		}

		// Default response location is body
		jsonName := f.JsonName
		if jsonName == "" {
			jsonName = f.Name
		}
		return jsonName, FieldLocationBody
	}

	// For request, location depends on method and tags
	isGetLike := method == "GET" || method == "HEAD" || method == "DELETE"

	// Check for explicit wire location
	if f.Wire != nil {
		if f.Wire.GetHeader() != nil {
			name := f.Wire.GetHeader().GetName()
			if name == "" {
				name = f.Name
			}
			return name, FieldLocationHeader
		} else if f.Wire.GetQuery() != nil {
			name := f.Wire.GetQuery().GetName()
			if name == "" {
				name = f.Name
			}
			return name, FieldLocationQuery
		}
	}

	// Check for Cookie
	for _, tag := range f.Tags {
		if tag.Key == "cookie" {
			name := tag.Name
			if name == "" {
				name = f.Name
			}
			return name, FieldLocationCookie
		}
	}

	// For GET-like methods, fields go in query by default
	if isGetLike {
		name := f.QueryStringName
		if name == "-" {
			return f.Name, FieldLocationUnused
		} else if name == "" {
			name = f.Name
		}
		return name, FieldLocationQuery
	}

	// Default request location for POST/PUT/PATCH is body
	jsonName := f.JsonName
	if jsonName == "-" {
		return f.Name, FieldLocationUnused
	} else if jsonName == "" {
		jsonName = f.Name
	}
	return jsonName, FieldLocationBody
}

// NamedOrInlineStruct returns the struct type and type arguments for a named or inline struct.
// Returns nil if the type is neither a named struct nor an inline struct.
func NamedOrInlineStruct(meta map[uint32]*schema.Decl, t *schema.Type) (*schema.Struct, []*schema.Type) {
	if t == nil {
		return nil, nil
	}

	if named := t.GetNamed(); named != nil {
		st := meta[named.Id]
		if st != nil && st.GetType() != nil {
			if structType := st.GetType().GetStruct(); structType != nil {
				return structType, named.GetTypeArguments()
			}
		}
	} else if structType := t.GetStruct(); structType != nil {
		return structType, []*schema.Type{}
	}

	return nil, nil
}
