package apienc

import (
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
	"golang.org/x/exp/slices"

	"encr.dev/pkg/idents"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	"encr.dev/v2/parser/apis/api"
)

// WireLoc is the location of a parameter in the HTTP request/response.
type WireLoc string

const (
	Header WireLoc = "header" // Parameter is placed in the HTTP header
	Query  WireLoc = "query"  // Parameter is placed in the query string
	Body   WireLoc = "body"   // Parameter is placed in the body
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

// DescribeAPI expresses how to encode an API's request and response objects for the wire.
func DescribeAPI(errs *perr.List, endpoint *api.Endpoint) *APIEncoding {
	encoding := &APIEncoding{
		DefaultMethod: defaultClientHTTPMethod(endpoint.HTTPMethods...),
	}

	encoding.RequestEncoding = DescribeRequest(errs, endpoint.Request, endpoint.HTTPMethods...)
	encoding.ResponseEncoding = DescribeResponse(errs, endpoint.Response)

	if encoding.RequestEncoding != nil {
		// Setup the default request encoding
		defaultEncoding := encoding.RequestEncodingForMethod(encoding.DefaultMethod)
		encoding.DefaultRequestEncoding = &RequestEncoding{
			HTTPMethods:      []string{encoding.DefaultMethod},
			HeaderParameters: defaultEncoding.HeaderParameters,
			BodyParameters:   defaultEncoding.BodyParameters,
			QueryParameters:  defaultEncoding.QueryParameters,
		}
	}

	return encoding
}

// defaultClientHTTPMethod works out the default HTTP method a client should use for a given RPC.
// When possible we will default to POST either when no method has been specified on the API or when
// then is a selection of methods and POST is one of them. If POST is not allowed as a method then
// we will use the first specified method.
func defaultClientHTTPMethod(httpMethods ...string) string {
	if httpMethods[0] == "*" {
		return "POST"
	}

	for _, httpMethod := range httpMethods {
		if httpMethod == "POST" {
			return "POST"
		}
	}
	return httpMethods[0]
}

// DescribeResponse generates a ParameterEncoding per field of the response struct and returns it as
// the ResponseEncoding
func DescribeResponse(errs *perr.List, responseSchema schema.Type) *ResponseEncoding {
	if responseSchema == nil {
		return &ResponseEncoding{}
	}

	responseStruct, ok := getConcreteNamedStruct(responseSchema)
	if !ok {
		errs.Addf(responseSchema.ASTExpr().Pos(), "API response type must be a named struct")
		return &ResponseEncoding{}
	}

	fields, err := describeParams(&encodingHints{Body, responseTags}, responseStruct)
	if err != nil {
		errs.Addf(responseSchema.ASTExpr().Pos(), "API response type is invalid: %v", err)
		return &ResponseEncoding{}
	}

	if keys := keyDiff(fields, Header, Body); len(keys) > 0 {
		errs.Addf(responseSchema.ASTExpr().Pos(), "API response must only contain body and header parameters, found %v", keys)
		return &ResponseEncoding{}
	}

	return &ResponseEncoding{
		BodyParameters:   fields[Body],
		HeaderParameters: fields[Header],
	}
}

func getConcreteNamedStruct(typ schema.Type) (st schema.StructType, ok bool) {
	if res, ok := schemautil.ResolveNamedStruct(typ, false); ok {
		concrete := schemautil.ConcretizeWithTypeArgs(res.Decl.Type, res.TypeArgs)
		return concrete.(schema.StructType), true
	}
	return schema.StructType{}, false
}

// keyDiff returns the diff between src.keys and keys
func keyDiff[T comparable, V any](src map[T]V, keys ...T) (diff []T) {
	for k, _ := range src {
		if !slices.Contains(keys, k) {
			diff = append(diff, k)
		}
	}
	return diff
}

// DescribeRequest groups the provided httpMethods by default WireLoc and returns a RequestEncoding
// per WireLoc
func DescribeRequest(errs *perr.List, requestSchema schema.Type, httpMethods ...string) []*RequestEncoding {
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

	st, ok := getConcreteNamedStruct(requestSchema)
	if !ok {
		errs.Addf(requestSchema.ASTExpr().Pos(), "API request type must be a named struct")
		return nil
	}

	var reqs []*RequestEncoding
	for location, methods := range methodsByDefaultLocation {
		var fields map[WireLoc][]*ParameterEncoding

		fields, err := describeParams(&encodingHints{location, requestTags}, st)
		if err != nil {
			errs.Addf(requestSchema.ASTExpr().Pos(), "API request type is invalid: %v", err)
			return nil
		}

		if keys := keyDiff(fields, Query, Header, Body); len(keys) > 0 {
			errs.Addf(requestSchema.ASTExpr().Pos(), "API response must only contain query, body, and header parameters, found %v", keys)
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

// describeParams calls describeParam() for each field in the payload struct
func describeParams(encodingHints *encodingHints, payload schema.StructType) (fields map[WireLoc][]*ParameterEncoding, err error) {
	paramByLocation := make(map[WireLoc][]*ParameterEncoding)
	for _, f := range payload.Fields {
		f, err := describeParam(encodingHints, f)
		if err != nil {
			return nil, err
		}

		if f != nil {
			paramByLocation[f.Location] = append(paramByLocation[f.Location], f)
		}
	}
	return paramByLocation, nil
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
func IgnoreField(field schema.StructField) bool {
	for _, tag := range field.Tag.Tags() {
		if _, found := requestTags[tag.Key]; found && tag.Name == "-" {
			return true
		}
	}
	return false
}

// describeParam returns the ParameterEncoding which uses field tags to describe how the parameter
// (e.g. qs, query, header) should be encoded in HTTP (name and location).
//
// It returns nil, nil if the field is not to be encoded.
func describeParam(encodingHints *encodingHints, field schema.StructField) (*ParameterEncoding, error) {
	if !field.Name.IsPresent() {
		// TODO(andre) We don't yet support encoding anonymous fields.
		return nil, errors.New("anonymous fields in top-level request/response types are not supported")
	}
	srcName := field.Name.Value

	defaultWireName := formatName(encodingHints.defaultLocation, field.Name.Value)
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
		tagHint, ok := encodingHints.tags[tag.Key]
		if !ok {
			continue
		}

		// If the presence of this tag overrides the default, update the location.
		if tagHint.overrideDefault {
			if usedOverrideTag != "" {
				// There is only allowed to be a single override.
				return nil, errors.Newf("tag conflict: %s cannot be combined with %s", usedOverrideTag, tag.Key)
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
				return nil, nil
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
	return &param, nil
}
