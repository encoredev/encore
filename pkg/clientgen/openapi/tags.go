package openapi

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	schema "encr.dev/proto/encore/parser/schema/v1"
)

func decorateSchemaRef(ref *openapi3.SchemaRef, apply func(*openapi3.Schema)) *openapi3.SchemaRef {
	if ref == nil {
		return ref
	}

	if ref.Value != nil {
		apply(ref.Value)
		return ref
	}

	if ref.Ref == "" {
		return ref
	}

	wrapper := &openapi3.Schema{AllOf: []*openapi3.SchemaRef{ref}}
	apply(wrapper)

	return &openapi3.SchemaRef{Value: wrapper}
}

func applyOpenAPIRawTag(ref *openapi3.SchemaRef, raw string) *openapi3.SchemaRef {
	if raw == "" {
		return ref
	}
	val, ok := reflect.StructTag(raw).Lookup("openapi")
	if !ok {
		return ref
	}
	return applyOpenAPITagParts(ref, []string{val})
}

func applyOpenAPITags(ref *openapi3.SchemaRef, tags []*schema.Tag) *openapi3.SchemaRef {
	for _, tag := range tags {
		if tag.GetKey() == "openapi" {
			return applyOpenAPITagParts(ref, append([]string{tag.GetName()}, tag.GetOptions()...))
		}
	}
	return ref
}

func applyOpenAPITagParts(ref *openapi3.SchemaRef, parts []string) *openapi3.SchemaRef {
	settings := parseOpenAPISettings(parts)
	if len(settings) == 0 {
		return ref
	}
	return decorateSchemaRef(ref, func(s *openapi3.Schema) { applyOpenAPISettings(s, settings) })
}

func parseOpenAPISettings(parts []string) map[string]string {
	settings := make(map[string]string)
	for _, part := range parts {
		for _, p := range splitOpenAPISettings(part) {
			p = strings.TrimSpace(p)
			if p == "" || p == "-" {
				continue
			}
			key, val, ok := strings.Cut(p, "=")
			if !ok {
				settings[p] = "true"
				continue
			}
			settings[strings.TrimSpace(key)] = strings.TrimSpace(val)
		}
	}
	return settings
}

func splitOpenAPISettings(s string) []string {
	var parts []string
	start, depth := 0, 0
	inString, escape := false, false
	for i, r := range s {
		switch {
		case escape:
			escape = false
		case r == '\\' && inString:
			escape = true
		case r == '"':
			inString = !inString
		case !inString && (r == '[' || r == '{' || r == '('):
			depth++
		case !inString && depth > 0 && (r == ']' || r == '}' || r == ')'):
			depth--
		case !inString && depth == 0 && (r == ';' || r == ','):
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

func applyOpenAPISettings(s *openapi3.Schema, settings map[string]string) {
	for key, val := range settings {
		switch key {
		case "example":
			s.Example = parseOpenAPIValue(val)
		case "default":
			s.Default = parseOpenAPIValue(val)
		case "format":
			s.Format = val
		case "deprecated":
			s.Deprecated = parseBoolDefaultTrue(val)
		case "nullable":
			s.Nullable = parseBoolDefaultTrue(val)
		case "title":
			s.Title = val
		case "readOnly":
			s.ReadOnly = parseBoolDefaultTrue(val)
		case "writeOnly":
			s.WriteOnly = parseBoolDefaultTrue(val)
		case "multipleOf":
			if f, ok := parseFloat(val); ok {
				s.MultipleOf = &f
			}
		case "enum":
			if vals := parseEnumValues(val); len(vals) > 0 {
				s.Enum = vals
			}
		case "minimum", "min":
			if f, ok := parseFloat(val); ok {
				s.Min = &f
			}
		case "maximum", "max":
			if f, ok := parseFloat(val); ok {
				s.Max = &f
			}
		case "minLength":
			if n, ok := parseUint(val); ok {
				s.MinLength = n
			}
		case "maxLength":
			if n, ok := parseUint(val); ok {
				s.MaxLength = &n
			}
		case "pattern":
			s.Pattern = val
		}
	}
}

func parseOpenAPIValue(s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

func literalValues(vals []*schema.Literal) []any {
	out := make([]any, 0, len(vals))
	for _, val := range vals {
		switch v := val.GetValue().(type) {
		case *schema.Literal_Str:
			out = append(out, v.Str)
		case *schema.Literal_Boolean:
			out = append(out, v.Boolean)
		case *schema.Literal_Int:
			out = append(out, v.Int)
		case *schema.Literal_Float:
			out = append(out, v.Float)
		}
	}
	return out
}

func parseEnumValues(s string) []any {
	if s == "" || s == "true" {
		return nil
	}
	var vals []any
	if err := json.Unmarshal([]byte(s), &vals); err == nil {
		return vals
	}
	parts := strings.Split(s, "|")
	vals = make([]any, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			vals = append(vals, parseOpenAPIValue(p))
		}
	}
	return vals
}

func parseBoolDefaultTrue(s string) bool {
	if s == "" || s == "true" {
		return true
	}
	b, err := strconv.ParseBool(s)
	return err == nil && b
}

func parseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

func parseUint(s string) (uint64, bool) {
	n, err := strconv.ParseUint(s, 10, 64)
	return n, err == nil
}

func applyValidationExpr(s *openapi3.Schema, expr *schema.ValidationExpr) {
	if expr == nil {
		return
	}
	if rule := expr.GetRule(); rule != nil {
		applyValidationRule(s, rule)
		return
	}
	if and := expr.GetAnd(); and != nil {
		for _, child := range and.GetExprs() {
			applyValidationExpr(s, child)
		}
	}
	// OpenAPI can model OR with anyOf, but flattening constraints into an
	// existing schema can change semantics. Leave OR unchanged for compatibility.
}

func applyValidationRule(s *openapi3.Schema, rule *schema.ValidationRule) {
	switch rule.GetRule().(type) {
	case *schema.ValidationRule_MinLen:
		s.MinLength = rule.GetMinLen()
	case *schema.ValidationRule_MaxLen:
		v := rule.GetMaxLen()
		s.MaxLength = &v
	case *schema.ValidationRule_MinVal:
		v := rule.GetMinVal()
		s.Min = &v
	case *schema.ValidationRule_MaxVal:
		v := rule.GetMaxVal()
		s.Max = &v
	case *schema.ValidationRule_StartsWith:
		s.Pattern = combinePattern(s.Pattern, "^"+regexp.QuoteMeta(rule.GetStartsWith()))
	case *schema.ValidationRule_EndsWith:
		s.Pattern = combinePattern(s.Pattern, regexp.QuoteMeta(rule.GetEndsWith())+"$")
	case *schema.ValidationRule_MatchesRegexp:
		s.Pattern = combinePattern(s.Pattern, rule.GetMatchesRegexp())
	case *schema.ValidationRule_Is_:
		switch rule.GetIs() {
		case schema.ValidationRule_EMAIL:
			s.Format = "email"
		case schema.ValidationRule_URL:
			s.Format = "uri"
		}
	}
}

func combinePattern(existing, next string) string {
	if existing == "" {
		return next
	}
	if strings.HasPrefix(existing, "(?=") || strings.HasPrefix(existing, "(?=.*") {
		return existing + patternLookahead(next)
	}
	return patternLookahead(existing) + patternLookahead(next)
}

func patternLookahead(pattern string) string {
	if strings.HasPrefix(pattern, "^") || strings.HasSuffix(pattern, "$") {
		return "(?=" + pattern + ")"
	}
	return "(?=.*(?:" + pattern + "))"
}
