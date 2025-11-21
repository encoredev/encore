package apienc

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/idents"
	"encr.dev/pkg/option"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/directive"
)

// WireLoc is the location of a parameter in the HTTP request/response.
type WireLoc string

const (
	Undefined  WireLoc = "undefined"  // Parameter location is undefined
	Header     WireLoc = "header"     // Parameter is placed in the HTTP header
	Query      WireLoc = "query"      // Parameter is placed in the query string
	Body       WireLoc = "body"       // Parameter is placed in the body
	Cookie     WireLoc = "cookie"     // Parameter is placed in cookies
	HTTPStatus WireLoc = "httpstatus" // Parameter represents the HTTP status code
)

var (
	QueryTag = tagDescription{
		location:        Query,
		overrideDefault: true,
	}
	QsTag     = QueryTag
	HeaderTag = tagDescription{
		location:        Header,
		overrideDefault: true,
		wireFormatter:   strings.ToLower,
	}
	JSONTag = tagDescription{
		location:        Body,
		omitEmptyOption: "omitempty",
		overrideDefault: false,
	}
	CookieTag = tagDescription{
		location:        Cookie,
		omitEmptyOption: "omitempty",
		overrideDefault: true,
	}
	HTTPStatusTag = tagDescription{
		location:        HTTPStatus,
		overrideDefault: true,
	}
)

// requestTags is a description of tags used for requests
var requestTags = map[string]tagDescription{
	"query":  QueryTag,
	"qs":     QsTag,
	"header": HeaderTag,
	"json":   JSONTag,
}

// responseTags is a description of tags used for responses
var responseTags = map[string]tagDescription{
	"header": HeaderTag,
	"json":   JSONTag,
}

// authTags is a description of tags used for auth
var authTags = map[string]tagDescription{
	"query":  QueryTag,
	"header": HeaderTag,
	"cookie": CookieTag,
}

// tagDescription is used to map struct field tags to param locations
// if overrideDefault is set, tagDescription.location will be used instead of encodingHints.defaultLocation
// if the tag matches the paramLocation, the param name will be replaced with the
// tag name
type tagDescription struct {
	location        WireLoc
	overrideDefault bool
	omitEmptyOption string
	wireFormatter   func(name string) string
}

// encodingHints is used to determine the default location and applicable tag overrides for http
// request/response encoding
type encodingHints struct {
	defaultLocation WireLoc
	tags            map[string]tagDescription
}

// APIEncoding expresses how an RPC should be encoded on the wire for both the request and responses.
type APIEncoding struct {
	DefaultMethod string `json:"default_method"`
	// Expresses how the default request encoding and method should be
	// Note: DefaultRequestEncoding.HTTPMethods will always be a slice with length 1
	DefaultRequestEncoding *RequestEncoding `json:"request_encoding"`
	// Expresses all the different ways the request can be encoded for this RPC
	RequestEncoding []*RequestEncoding `json:"all_request_encodings"`
	// Expresses how the response to this RPC will be encoded
	ResponseEncoding *ResponseEncoding `json:"response_encoding"`
}

// RequestEncodingForMethod returns the request encoding required for the given HTTP method.
// If the method is not supported by the RPC it reports nil.
func (e *APIEncoding) RequestEncodingForMethod(method string) *RequestEncoding {
	var wildcardOption *RequestEncoding

	for _, reqEnc := range e.RequestEncoding {
		for _, m := range reqEnc.HTTPMethods {
			if strings.EqualFold(m, method) {
				return reqEnc
			}

			if m == "*" {
				wildcardOption = reqEnc
			}
		}
	}
	return wildcardOption
}

// ResponseEncoding expresses how a response should be encoded on the wire
type ResponseEncoding struct {
	// Contains metadata about how to marshal an HTTP parameter
	HeaderParameters []*ParameterEncoding `json:"header_parameters"`
	BodyParameters   []*ParameterEncoding `json:"body_parameters"`
	// HTTPStatusParameter contains encoding info for the HTTP status field, if any
	HTTPStatusParameter *ParameterEncoding `json:"http_status_parameter,omitempty"`
}

func (r *ResponseEncoding) AllParameters() []*ParameterEncoding {
	params := append(r.HeaderParameters, r.BodyParameters...)
	if r.HTTPStatusParameter != nil {
		params = append(params, r.HTTPStatusParameter)
	}
	return params
}

// RequestEncoding expresses how a request should be encoded for an explicit set of HTTPMethods
type RequestEncoding struct {
	// The HTTP methods this encoding can be used for
	HTTPMethods []string `json:"http_methods"`
	// Contains metadata about how to marshal an HTTP parameter
	HeaderParameters []*ParameterEncoding `json:"header_parameters"`
	QueryParameters  []*ParameterEncoding `json:"query_parameters"`
	BodyParameters   []*ParameterEncoding `json:"body_parameters"`
}

func (r *RequestEncoding) AllParameters() []*ParameterEncoding {
	return append(append(r.HeaderParameters, r.QueryParameters...), r.BodyParameters...)
}

// ParameterEncoding expresses how a parameter should be encoded on the wire
type ParameterEncoding struct {
	// SrcName is the name of the struct field
	SrcName string `json:"src_name"`
	// WireName is how the name is encoded on the wire.
	WireName string `json:"wire_name"`
	// Location is the location this encoding is for.
	Location WireLoc `json:"location"`
	// OmitEmpty specifies whether the parameter should be omitted if it's empty.
	OmitEmpty bool `json:"omit_empty"`
	// Doc is the documentation of the struct field
	Doc string `json:"doc"`
	// Type is the field's type description.
	Type schema.Type `json:"type"`
}

// DescribeResponse generates a ParameterEncoding per field of the response struct and returns it as
// the ResponseEncoding
func DescribeResponse(errs *perr.List, responseSchema schema.Type) *ResponseEncoding {
	if responseSchema == nil {
		return &ResponseEncoding{}
	}

	responseStruct, ok := getConcreteNamedStruct(errs, responseSchema)
	if !ok {
		errs.Add(errResponseMustBeNamedStruct.AtGoNode(responseSchema.ASTExpr()))
		return &ResponseEncoding{}
	}

	fields, ok := describeParams(errs, &encodingHints{Body, responseTags}, responseStruct)
	if !ok {
		// describeParams already added the error to errs
		return &ResponseEncoding{}
	}

	// Extract HTTP status parameter from fields and check for multiple status fields
	var httpStatusParameter *ParameterEncoding
	if len(fields[HTTPStatus]) > 0 {
		switch len(fields[HTTPStatus]) {
		case 1:
			httpStatusParameter = fields[HTTPStatus][0]
		default:
			errs.Add(errMultipleHTTPStatusFields.AtGoNode(responseStruct.ASTExpr()))
		}
	}

	// Check for reserved header prefixes
	for _, field := range fields[Header] {
		if strings.HasPrefix(strings.ToLower(field.WireName), "x-encore-") {
			errs.Add(errReservedHeaderPrefix.AtGoNode(field.Type.ASTExpr()))
		}
	}

	if keys := keyDiff(fields, Header, Body, HTTPStatus); len(keys) > 0 {
		err := errResponseTypeMustOnlyBeBodyOrHeaders.AtGoNode(responseSchema.ASTExpr())

		for _, k := range keys {
			for _, field := range fields[k] {
				err = err.AtGoNode(field.Type.ASTExpr(), errors.AsError(fmt.Sprintf("found %s", field.Location)))
			}
		}
		errs.Add(err)
		return &ResponseEncoding{}
	}

	return &ResponseEncoding{
		BodyParameters:      fields[Body],
		HeaderParameters:    fields[Header],
		HTTPStatusParameter: httpStatusParameter,
	}
}

func getConcreteNamedStruct(errs *perr.List, typ schema.Type) (st schema.StructType, ok bool) {
	if res, ok := schemautil.ResolveNamedStruct(typ, false); ok {
		concrete := schemautil.ConcretizeWithTypeArgs(errs, res.Decl.Type, res.TypeArgs)
		return concrete.(schema.StructType), true
	}
	return schema.StructType{}, false
}

// keyDiff returns the diff between src.keys and keys
func keyDiff[T comparable, V any](src map[T]V, keys ...T) (diff []T) {
	for k := range src {
		if !slices.Contains(keys, k) {
			diff = append(diff, k)
		}
	}
	return diff
}

// DescribeRequest groups the provided httpMethods by default WireLoc and returns a RequestEncoding
// per WireLoc
func DescribeRequest(errs *perr.List, requestAST schema.Param, requestSchema schema.Type, methodsField option.Option[directive.Field], httpMethods ...string) []*RequestEncoding {
	methodsByDefaultLocation := make(map[WireLoc][]string)
	for _, m := range httpMethods {
		switch m {
		case "GET", "HEAD", "DELETE":
			methodsByDefaultLocation[Query] = append(methodsByDefaultLocation[Query], m)
		case "*":
			methodsByDefaultLocation[Body] = []string{"POST", "PUT", "PATCH"}
			methodsByDefaultLocation[Query] = []string{"GET", "HEAD", "DELETE"}
		default:
			methodsByDefaultLocation[Body] = append(methodsByDefaultLocation[Body], m)
		}
	}

	if requestSchema == nil {
		// If there is no request schema, add an empty encoding that supports
		// all methods allowed by the endpoint.
		enc := &RequestEncoding{
			HTTPMethods: httpMethods,
		}
		if len(httpMethods) > 0 && httpMethods[0] == "*" {
			enc.HTTPMethods = []string{"POST", "PUT", "PATCH", "GET", "HEAD", "DELETE"}
		}
		return []*RequestEncoding{enc}
	}

	st, ok := getConcreteNamedStruct(errs, requestSchema)
	if !ok {
		errs.Add(errRequestMustBeNamedStruct.AtGoNode(requestSchema.ASTExpr()))
		return nil
	}

	var reqs []*RequestEncoding
	for location, methods := range methodsByDefaultLocation {
		var fields map[WireLoc][]*ParameterEncoding

		fields, ok := describeParams(errs, &encodingHints{location, requestTags}, st)
		if !ok {
			// report error in describeParams
			return nil
		}

		// Check for reserved header prefixes or invalid data types
		for _, field := range fields[Header] {
			if strings.HasPrefix(strings.ToLower(field.WireName), "x-encore-") {
				errs.Add(errReservedHeaderPrefix.AtGoNode(field.Type.ASTExpr()))
			}

			if !schemautil.IsValidHeaderType(field.Type) {
				errs.Add(
					errInvalidHeaderType(field.Type.String()).
						AtGoNode(field.Type.ASTExpr(), errors.AsError("unsupported type")).
						AtGoNode(requestAST.AST, errors.AsHelp("used here")),
				)
			}
		}

		// Check for invalid datatype in query parameters
		for _, field := range fields[Query] {
			if !schemautil.IsValidQueryType(field.Type) {
				err := errInvalidQueryStringType(field.Type.String()).
					AtGoNode(field.Type.ASTExpr(), errors.AsError("unsupported type")).
					AtGoNode(requestAST.AST, errors.AsHelp("used here"))

				if field, ok := methodsField.Get(); ok {
					httpMethod := strings.ToUpper(field.Value)
					switch httpMethod {
					case "GET", "HEAD", "DELETE":
						err = err.AtGoNode(field, errors.AsHelp("you could change this to a POST or PUT request"))
					}
				}

				errs.Add(err)
			}
		}

		if errs.Len() > 0 {
			return nil
		}

		if keys := keyDiff(fields, Query, Header, Body); len(keys) > 0 {
			err := errRequestInvalidLocation.AtGoNode(requestSchema.ASTExpr())

			for _, k := range keys {
				for _, field := range fields[k] {
					err = err.AtGoNode(field.Type.ASTExpr(), errors.AsError(fmt.Sprintf("found %s", field.Location)))
				}
			}
			errs.Add(err)
			return nil
		}
		reqs = append(reqs, &RequestEncoding{
			HTTPMethods:      methods,
			QueryParameters:  fields[Query],
			HeaderParameters: fields[Header],
			BodyParameters:   fields[Body],
		})
	}

	// Sort by first method to get a deterministic order (list is randomized by map above)
	sort.Slice(reqs, func(i, j int) bool {
		return reqs[i].HTTPMethods[0] < reqs[j].HTTPMethods[0]
	})
	return reqs
}

// AuthEncoding expresses how a response should be encoded on the wire.
type AuthEncoding struct {
	// LegacyTokenFormat specifies whether the auth encoding uses the legacy format of
	// "just give us a token as a string". If true, the other parameters are all empty.
	LegacyTokenFormat bool

	// Contains metadata about how to marshal an HTTP parameter
	HeaderParameters []*ParameterEncoding `json:"header_parameters"`
	QueryParameters  []*ParameterEncoding `json:"query_parameters"`
	CookieParameters []*ParameterEncoding `json:"cookie_parameters"`
}

// DescribeAuth generates a ParameterEncoding per field of the auth struct and returns it as
// the AuthEncoding. If authSchema is nil it returns nil.
func DescribeAuth(errs *perr.List, authSchema schema.Type) *AuthEncoding {
	if authSchema == nil {
		return nil
	}

	// Do we have a legacy, string-based handler?
	if builtin, ok := authSchema.(schema.BuiltinType); ok {
		if builtin.Kind != schema.String {
			errs.Add(authhandler.ErrInvalidAuthSchemaType.AtGoNode(builtin.ASTExpr()))
		}
		return &AuthEncoding{LegacyTokenFormat: true}
	}

	st, ok := getConcreteNamedStruct(errs, authSchema)
	if !ok {
		errs.Add(authhandler.ErrInvalidAuthSchemaType.AtGoNode(authSchema.ASTExpr()))
		return nil
	}

	fields, ok := describeParams(errs, &encodingHints{Undefined, authTags}, st)
	if !ok {
		// reported by describeParams
		return nil
	}
	if locationDiff := keyDiff(fields, Header, Query, Cookie); len(locationDiff) > 0 {
		err := authhandler.ErrInvalidFieldTags.AtGoNode(authSchema.ASTExpr())

		for _, k := range locationDiff {
			for _, field := range fields[k] {
				err = err.AtGoNode(field.Type.ASTExpr(), errors.AsError(fmt.Sprintf("found %s", field.Location)))
			}
		}
		errs.Add(err)
		return nil
	}
	return &AuthEncoding{
		QueryParameters:  fields[Query],
		HeaderParameters: fields[Header],
		CookieParameters: fields[Cookie],
	}
}

// describeParams calls describeParam() for each field in the payload struct
func describeParams(errs *perr.List, encodingHints *encodingHints, payload schema.StructType) (fields map[WireLoc][]*ParameterEncoding, ok bool) {
	paramByLocation := make(map[WireLoc][]*ParameterEncoding)
	for _, f := range payload.Fields {
		if !f.IsExported() {
			continue
		}

		f, ok := describeParam(errs, encodingHints, f)
		if !ok {
			return nil, false
		}

		if f != nil {
			paramByLocation[f.Location] = append(paramByLocation[f.Location], f)
		}
	}
	return paramByLocation, true
}

// formatName formats a parameter name with the default formatting for the location (e.g. snakecase for query)
func formatName(location WireLoc, name string) string {
	switch location {
	case Query:
		return idents.Convert(name, idents.SnakeCase)
	default:
		return name
	}
}

// IgnoreField returns true if the field name is "-" is any of the valid request or response tags
// or if the field is marked with encore:"httpstatus" (which shouldn't appear in client types)
func IgnoreField(field schema.StructField) bool {
	for _, tag := range field.Tag.Tags() {
		if _, found := requestTags[tag.Key]; found && tag.Name == "-" {
			return true
		}
		// Skip fields with encore:"httpstatus" tag - they're for internal HTTP status handling only
		if tag.Key == "encore" && tag.Name == "httpstatus" {
			return true
		}
	}
	return false
}

// describeParam returns the ParameterEncoding which uses field tags to describe how the parameter
// (e.g. qs, query, header) should be encoded in HTTP (name and location).
//
// It returns nil, nil if the field is not to be encoded.
func describeParam(errs *perr.List, encodingHints *encodingHints, field schema.StructField) (*ParameterEncoding, bool) {
	if field.Name.Empty() {
		// TODO(andre) We don't yet support encoding anonymous fields.
		errs.Add(errAnonymousFields.AtGoNode(field.AST))
		return nil, false
	}
	srcName := field.Name.MustGet()

	defaultWireName := formatName(encodingHints.defaultLocation, srcName)
	param := ParameterEncoding{
		OmitEmpty: false,
		SrcName:   srcName,
		Doc:       field.Doc,
		Type:      field.Type,
		WireName:  defaultWireName,
	}

	// Determine which location we should use for this field.
	location := encodingHints.defaultLocation
	var usedOverrideTag string
	for _, tag := range field.Tag.Tags() {
		// Handle fields with encore:"httpstatus" tag
		if tag.Key == "encore" && tag.Name == "httpstatus" {
			if !isValidHTTPStatusType(field.Type) {
				errs.Add(errHTTPStatusFieldMustBeInt.AtGoNode(field.AST))
				return nil, false
			}

			return &ParameterEncoding{
				SrcName:   srcName,
				WireName:  "httpstatus",
				Location:  HTTPStatus,
				OmitEmpty: false,
				Doc:       field.Doc,
				Type:      field.Type,
			}, true
		}

		tagHint, ok := encodingHints.tags[tag.Key]
		if !ok {
			continue
		}

		// If the presence of this tag overrides the default, update the location.
		if tagHint.overrideDefault {
			if usedOverrideTag != "" {
				// There is only allowed to be a single override.
				errs.Add(errTagConflict(usedOverrideTag, tag.Key).AtGoNode(field.AST.Tag))
				return nil, false
			}
			location = tagHint.location
			usedOverrideTag = tag.Key
		}
	}

	// For the location, see if there is tag information for it.
	for _, tag := range field.Tag.Tags() {
		if tagHint, ok := encodingHints.tags[tag.Key]; ok && tagHint.location == location {
			// We have found the tag that applies to the default location.
			// Determine if this tag actually has a name. If not, use the existing name.
			if tag.Name == "-" {
				// This field is to be ignored.
				return nil, true
			}
			if tag.Name != "" {
				if tagHint.wireFormatter != nil {
					param.WireName = tagHint.wireFormatter(tag.Name)
				} else {
					param.WireName = tag.Name
				}
			}
			param.OmitEmpty = tag.HasOption("omitempty")
		}
	}

	param.Location = location
	return &param, true
}

// isValidHTTPStatusType returns true if the given type is valid for HTTP status fields.
// Valid types are integer types that can hold a http status code
func isValidHTTPStatusType(typ schema.Type) bool {
	builtin, ok := typ.(schema.BuiltinType)
	if !ok {
		return false
	}

	switch builtin.Kind {
	case schema.Int, schema.Int16, schema.Int32, schema.Int64,
		schema.Uint, schema.Uint16, schema.Uint32, schema.Uint64:
		return true
	default:
		return false
	}
}
