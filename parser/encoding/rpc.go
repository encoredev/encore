package encoding

import (
	"reflect"
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/golang/protobuf/proto"

	"golang.org/x/exp/slices"

	"encr.dev/parser"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// ParameterLocation is the request/response home of the parameter
type ParameterLocation int

const (
	Header ParameterLocation = iota // Parameter is placed in the HTTP header
	Query                           // Parameter is placed in the query string
	Body                            // Parameter is placed in the body
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
		nameFormatter:   strings.ToLower,
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
	location        ParameterLocation
	overrideDefault bool
	omitEmptyOption string
	nameFormatter   func(string) string
}

// encodingHints is used to determine the default location and applicable tag overrides for http
// request/response encoding
type encodingHints struct {
	defaultLocation ParameterLocation
	tags            map[string]tagDescription
	options         *Options
}

// RPCEncoding expresses how an RPC should be encoded on the wire for both the request and responses.
type RPCEncoding struct {
	// Expresses how the default request encoding and method should be
	// Note: DefaultRequestEncoding.HTTPMethods will always be a slice with length 1
	DefaultRequestEncoding *RequestEncoding `json:"default_request_encoding"`
	// Expresses all the different ways the request can be encoded for this RPC
	RequestEncoding []*RequestEncoding `json:"request_encoding"`
	// Expresses how the response to this RPC will be encoded
	ResponseEncoding *ResponseEncoding `json:"response_encoding"`
}

// RequestEncodingForMethod returns the request encoding required for the given HTTP method
func (e *RPCEncoding) RequestEncodingForMethod(method string) *RequestEncoding {
	for _, reqEnc := range e.RequestEncoding {
		for _, m := range reqEnc.HTTPMethods {
			if m == method {
				return reqEnc
			}
		}
	}
	return nil
}

// ResponseEncoding expresses how a response should be encoded on the wire
type ResponseEncoding struct {
	// Contains metadata about how to marshal an HTTP parameter
	BodyParameters   []*ParameterEncoding `json:"body_parameters"`
	HeaderParameters []*ParameterEncoding `json:"header_parameters"`
}

// RequestEncoding expresses how a request should be encoded for an explicit set of HTTPMethods
type RequestEncoding struct {
	// The HTTP methods these field configurations can be used for
	HTTPMethods []string `json:"http_methods"`
	// Contains metadata about how to marshal an HTTP parameter
	BodyParameters   []*ParameterEncoding `json:"body_parameters"`
	HeaderParameters []*ParameterEncoding `json:"header_parameters"`
	QueryParameters  []*ParameterEncoding `json:"query_parameters"`
}

// ParameterEncoding expresses how a parameter should be encoded on the wire
type ParameterEncoding struct {
	// The location specific name of the parameter (e.g. cheeseEater, cheese-eater, X-Cheese-Eater
	Name string `json:"name"`
	// Whether the parameter should be omitted if it's empty
	OmitEmpty bool `json:"omit_empty"`
	// The name of the struct field
	SrcName string `json:"src_name"`
	// Doc of the struct field
	Doc string `json:"doc"`
	// The field type
	Type *schema.Type `json:"type"`
	// The raw tag of the field
	RawTag string `json:"-"`
}

type Options struct {
	SrcNameTag string
}

// DescribeRPC expresses how to encode an RPCs request and response objects for the wire.
func DescribeRPC(appMetaData *meta.Data, rpc *meta.RPC, options *Options) (*RPCEncoding, error) {
	encoding := &RPCEncoding{}
	var err error
	// Work out the request encoding
	encoding.RequestEncoding, err = DescribeRequest(appMetaData, rpc.RequestSchema, options, rpc.HttpMethods...)
	if err != nil {
		return nil, errors.Wrap(err, "request encoding")
	}

	// Work out the response encoding
	encoding.ResponseEncoding, err = DescribeResponse(appMetaData, rpc.ResponseSchema, options)
	if err != nil {
		return nil, errors.Wrap(err, "request encoding")
	}

	// Setup the default request encoding
	defaultMethod := DefaultClientHttpMethod(rpc)
	defaultEncoding := encoding.RequestEncodingForMethod(defaultMethod)
	encoding.DefaultRequestEncoding = &RequestEncoding{
		HTTPMethods:      []string{defaultMethod},
		HeaderParameters: defaultEncoding.HeaderParameters,
		BodyParameters:   defaultEncoding.BodyParameters,
		QueryParameters:  defaultEncoding.QueryParameters,
	}

	return encoding, nil
}

// getConcreteStructType returns a construct Struct object for the given schema. This means any generic types
// in the struct will be resolved to their concrete types and there will be no generic parameters in the struct object.
// However, any nested structs may still contain generic types.
//
// If a nil schema is provided, a nil struct is returned.
func getConcreteStructType(appMetaData *meta.Data, typ *schema.Type, typeArgs []*schema.Type) (*schema.Struct, error) {
	if typ == nil {
		// If there's no schema type, we want to shortcut
		return nil, nil
	}

	switch typ := typ.Typ.(type) {
	case *schema.Type_Struct:
		// If there are no type arguments, we've got a concrete type
		if len(typeArgs) == 0 {
			return typ.Struct, nil
		}

		// Deep copy the original struct
		struc, ok := proto.Clone(typ.Struct).(*schema.Struct)
		if !ok {
			return nil, errors.New("failed to clone struct")
		}

		// replace any type parameters with the type argument
		for _, field := range struc.Fields {
			field.Typ = resolveTypeParams(field.Typ, typeArgs)
		}

		return struc, nil
	case *schema.Type_Named:
		decl := appMetaData.Decls[typ.Named.Id]
		return getConcreteStructType(appMetaData, decl.Type, typ.Named.TypeArguments)
	default:
		return nil, errors.Newf("unsupported type %+v", reflect.TypeOf(typ))
	}
}

// resolveTypeParams resolves any type parameters in the given type to the given type arguments.
// only at the top level object - so nested type arguments are not resolved
func resolveTypeParams(typ *schema.Type, typeArgs []*schema.Type) *schema.Type {
	switch t := typ.Typ.(type) {
	case *schema.Type_TypeParameter:
		return typeArgs[t.TypeParameter.ParamIdx]

	case *schema.Type_List:
		t.List.Elem = resolveTypeParams(t.List.Elem, typeArgs)

	case *schema.Type_Map:
		t.Map.Key = resolveTypeParams(t.Map.Key, typeArgs)
		t.Map.Value = resolveTypeParams(t.Map.Value, typeArgs)

	case *schema.Type_Named:
		for i, param := range t.Named.TypeArguments {
			t.Named.TypeArguments[i] = resolveTypeParams(param, typeArgs)
		}
	}

	return typ
}

// DefaultClientHttpMethod works out the default HTTP method a client should use for a given RPC.
// When possible we will default to POST either when no method has been specified on the API or when
// then is a selection of methods and POST is one of them. If POST is not allowed as a method then
// we will use the first specified method.
func DefaultClientHttpMethod(rpc *meta.RPC) string {
	if rpc.HttpMethods[0] == "*" {
		return "POST"
	}

	for _, httpMethod := range rpc.HttpMethods {
		if httpMethod == "POST" {
			return "POST"
		}
	}

	return rpc.HttpMethods[0]

}

// DescribeResponse generates a ParameterEncoding per field of the response struct and returns it as
// the ResponseEncoding
func DescribeResponse(appMetaData *meta.Data, responseSchema *schema.Type, options *Options) (*ResponseEncoding, error) {
	responseStruct, err := getConcreteStructType(appMetaData, responseSchema, nil)
	if err != nil {
		return nil, errors.Wrap(err, "response struct")
	}
	fields, err := describeParams(&encodingHints{Body, responseTags, options}, responseStruct)
	if err != nil {
		return nil, err
	}
	if keys := keyDiff(fields, Header, Body); len(keys) > 0 {
		return nil, errors.Newf("response must only contain body and header parameters. Found: %v", keys)
	}
	return &ResponseEncoding{
		BodyParameters:   fields[Body],
		HeaderParameters: fields[Header],
	}, nil
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

// DescribeRequest groups the provided httpMethods by default ParameterLocation and returns a RequestEncoding
// per ParameterLocation
func DescribeRequest(appMetaData *meta.Data, requestSchema *schema.Type, options *Options, httpMethods ...string) ([]*RequestEncoding, error) {
	requestStruct, err := getConcreteStructType(appMetaData, requestSchema, nil)
	if err != nil {
		return nil, errors.Wrap(err, "request struct")
	}
	methodsByDefaultLocation := make(map[ParameterLocation][]string)
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

	var reqs []*RequestEncoding
	for location, methods := range methodsByDefaultLocation {
		fields, err := describeParams(&encodingHints{location, requestTags, options}, requestStruct)
		if err != nil {
			return nil, err
		}
		if keys := keyDiff(fields, Query, Header, Body); len(keys) > 0 {
			return nil, errors.Newf("request must only contain Query, Body and Header parameters. Found: %v", keys)
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
	return reqs, nil
}

// describeParams calls describeParam() for each field in the payload struct
func describeParams(encodingHints *encodingHints, payload *schema.Struct) (fields map[ParameterLocation][]*ParameterEncoding, err error) {
	paramByLocation := make(map[ParameterLocation][]*ParameterEncoding)
	for _, f := range payload.GetFields() {
		location, f, err := describeParam(encodingHints, f)
		if err != nil {
			return nil, err
		}

		// fields explicitly named "-" are excluded from the generated client
		if f.Name != "-" {
			paramByLocation[location] = append(paramByLocation[location], f)
		}
	}
	return paramByLocation, nil
}

// formatName formats a parameter name with the default formatting for the location (e.g. snakecase for query)
func formatName(location ParameterLocation, name string) string {
	switch location {
	case Query:
		return parser.SnakeCase(name)
	default:
		return name
	}
}

// describeParam returns a ParameterLocation, ParameterEncoding  which uses field tags to describe how the parameter
//(e.g. qs, query, header) should be encoded in HTTP (name and location)
func describeParam(encodingHints *encodingHints, field *schema.Field) (ParameterLocation, *ParameterEncoding, error) {
	location := encodingHints.defaultLocation
	param := ParameterEncoding{
		Name:      formatName(encodingHints.defaultLocation, field.Name),
		OmitEmpty: false,
		SrcName:   field.Name,
		Doc:       field.Doc,
		Type:      field.Typ,
		RawTag:    field.RawTag,
	}

	var usedOverrideTag string
	for _, tag := range field.Tags {
		tagHint, ok := encodingHints.tags[tag.Key]
		if !ok {
			continue
		}
		if tagHint.overrideDefault {
			if usedOverrideTag != "" {
				return 0, nil, errors.Newf("tag conflict: %s cannot be combined with %s", usedOverrideTag, tag.Key)
			}
			location = tagHint.location
			usedOverrideTag = tag.Key
		}
		if tagHint.location == location {
			if tagHint.nameFormatter != nil {
				param.Name = tagHint.nameFormatter(tag.Name)
			} else {
				param.Name = tag.Name
			}
		}
		if tagHint.omitEmptyOption != "" {
			for _, o := range tag.Options {
				if o == tagHint.omitEmptyOption {
					param.OmitEmpty = true
				}
			}
		}
		if encodingHints.options != nil && tag.Key == encodingHints.options.SrcNameTag {
			param.SrcName = tag.Name
		}
	}
	return location, &param, nil
}
