package encoding

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/golang/protobuf/proto"

	"encr.dev/pkg/idents"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// ParameterLocation is the request/response home of the parameter
type ParameterLocation string

const (
	Undefined ParameterLocation = "undefined" // Parameter location is Undefined
	Header    ParameterLocation = "header"    // Parameter is placed in the HTTP header
	Query     ParameterLocation = "query"     // Parameter is placed in the query string
	Body      ParameterLocation = "body"      // Parameter is placed in the body
	Cookie    ParameterLocation = "cookie"    // Parameter is placed in cookies
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
)

// authTags is a description of tags used for auth
var authTags = map[string]tagDescription{
	"query":  QueryTag,
	"header": HeaderTag,
	"cookie": CookieTag,
}

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
	wireFormatter   func(name string) string
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
	Name        string     `json:"name"`
	Doc         string     `json:"doc"`
	AccessType  string     `json:"access_type"`
	Proto       string     `json:"proto"`
	Path        *meta.Path `json:"path"`
	HttpMethods []string   `json:"http_methods"`

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
func (e *RPCEncoding) RequestEncodingForMethod(method string) *RequestEncoding {
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

// ParameterEncodingMap returns the parameter encodings as a map, keyed by SrcName.
func (e *AuthEncoding) ParameterEncodingMap() map[string]*ParameterEncoding {
	return toEncodingMap(srcNameKey, e.HeaderParameters, e.QueryParameters, e.CookieParameters)
}

// ParameterEncodingMapByName returns the parameter encodings as a map, keyed by Name.
// Conflicts result in an undefined encoding getting set.
func (e *AuthEncoding) ParameterEncodingMapByName() map[string][]*ParameterEncoding {
	return toEncodingMultiMap(nameKey, e.HeaderParameters, e.QueryParameters, e.CookieParameters)
}

// ResponseEncoding expresses how a response should be encoded on the wire
type ResponseEncoding struct {
	// Contains metadata about how to marshal an HTTP parameter
	HeaderParameters []*ParameterEncoding `json:"header_parameters"`
	BodyParameters   []*ParameterEncoding `json:"body_parameters"`
}

// ParameterEncodingMap returns the parameter encodings as a map, keyed by SrcName.
func (e *ResponseEncoding) ParameterEncodingMap() map[string]*ParameterEncoding {
	return toEncodingMap(srcNameKey, e.HeaderParameters, e.BodyParameters)
}

// ParameterEncodingMapByName returns the parameter encodings as a map, keyed by Name.
// Conflicts result in an undefined encoding getting set.
func (e *ResponseEncoding) ParameterEncodingMapByName() map[string][]*ParameterEncoding {
	return toEncodingMultiMap(nameKey, e.HeaderParameters, e.BodyParameters)
}

// RequestEncoding expresses how a request should be encoded for an explicit set of HTTPMethods
type RequestEncoding struct {
	// The HTTP methods these field configurations can be used for
	HTTPMethods []string `json:"http_methods"`
	// Contains metadata about how to marshal an HTTP parameter
	HeaderParameters []*ParameterEncoding `json:"header_parameters"`
	QueryParameters  []*ParameterEncoding `json:"query_parameters"`
	BodyParameters   []*ParameterEncoding `json:"body_parameters"`
}

// ParameterEncodingMap returns the parameter encodings as a map, keyed by SrcName.
func (e *RequestEncoding) ParameterEncodingMap() map[string]*ParameterEncoding {
	return toEncodingMap(srcNameKey, e.HeaderParameters, e.QueryParameters, e.BodyParameters)
}

// ParameterEncodingMapByName returns the parameter encodings as a map, keyed by Name.
// Conflicts result in an undefined encoding getting set.
func (e *RequestEncoding) ParameterEncodingMapByName() map[string][]*ParameterEncoding {
	return toEncodingMultiMap(nameKey, e.HeaderParameters, e.QueryParameters, e.BodyParameters)
}

// ParameterEncoding expresses how a parameter should be encoded on the wire
type ParameterEncoding struct {
	// The location specific name of the parameter (e.g. cheeseEater, cheese-eater, X-Cheese-Eater)
	Name string `json:"name"`
	// Location is the location this encoding is for.
	Location ParameterLocation `json:"location"`
	// OmitEmpty specifies whether the parameter should be omitted if it's empty.
	OmitEmpty bool `json:"omit_empty"`
	// SrcName is the name of the struct field
	SrcName string `json:"src_name"`
	// Doc is the documentation of the struct field
	Doc string `json:"doc"`
	// Type is the field's type description.
	Type *schema.Type `json:"type"`
	// RawTag specifies the raw, unparsed struct tag for the field.
	RawTag string `json:"raw_tag"`
	// WireFormat is the wire format of the parameter.
	WireFormat string `json:"wire_format"`
	// Optional indicates whether the field is optional.
	Optional bool `json:"optional"`
}

type Options struct {
	// SrcNameTag, if set, specifies which source tag should be used to determine
	// the value of the SrcName field in the returned parameter descriptions.
	//
	// If the given SrcNameTag is not present on the field, SrcName will be set
	// to the Go field name instead.
	//
	// If SrcNameTag is empty, SrcName is set to the Go field name.
	SrcNameTag string
}

type APIEncoding struct {
	Services      []*ServiceEncoding `json:"services"`
	Authorization *AuthEncoding      `json:"authorization"`
}

type ServiceEncoding struct {
	Name string         `json:"name"`
	Doc  string         `json:"doc"`
	RPCs []*RPCEncoding `json:"rpcs"`
}

func DescribeAPI(meta *meta.Data) *APIEncoding {
	api := &APIEncoding{Services: make([]*ServiceEncoding, len(meta.Svcs))}
	for i, s := range meta.Svcs {
		api.Services[i] = DescribeService(meta, s)
	}
	if meta.AuthHandler == nil {
		return api
	}

	var err error
	api.Authorization, err = DescribeAuth(meta, meta.AuthHandler.Params, nil)
	if err != nil {
		panic(fmt.Sprintf("Invalid auth definition: %s: %v", meta.AuthHandler.Name, err))
	}
	return api
}

func findDoc(relPath string, meta *meta.Data) string {
	for _, p := range meta.Pkgs {
		if p.RelPath == relPath {
			return p.Doc
		}
	}
	return ""
}

func DescribeService(meta *meta.Data, svc *meta.Service) *ServiceEncoding {
	service := &ServiceEncoding{Name: svc.Name, Doc: findDoc(svc.RelPath, meta), RPCs: make([]*RPCEncoding, len(svc.Rpcs))}
	for i, r := range svc.Rpcs {
		rpc, err := DescribeRPC(meta, r, nil)
		if err != nil {
			panic(fmt.Sprintf("invalid rpc: %v", err))
		}
		service.RPCs[i] = rpc
	}
	return service
}

// DescribeRPC expresses how to encode an RPCs request and response objects for the wire.
func DescribeRPC(appMetaData *meta.Data, rpc *meta.RPC, options *Options) (*RPCEncoding, error) {
	encoding := &RPCEncoding{
		DefaultMethod: DefaultClientHttpMethod(rpc),
		Name:          rpc.Name,
		AccessType:    rpc.AccessType.String(),
		Proto:         rpc.Proto.String(),
		Path:          rpc.Path,
		Doc:           rpc.GetDoc(),
	}
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

	return encoding, nil
}

// GetConcreteStructType returns a construct Struct object for the given schema. This means any generic types
// in the struct will be resolved to their concrete types and there will be no generic parameters in the struct object.
// However, any nested structs may still contain generic types.
//
// If a nil schema is provided, a nil struct is returned.
func GetConcreteStructType(appDecls []*schema.Decl, typ *schema.Type, typeArgs []*schema.Type) (*schema.Struct, error) {
	// dereference pointers
	pointer := typ.GetPointer()
	for pointer != nil {
		typ = pointer.Base
		pointer = typ.GetPointer()
	}

	typ, err := GetConcreteType(appDecls, typ, typeArgs)
	if err != nil {
		return nil, err
	}

	struc := typ.GetStruct()
	if struc == nil {
		return nil, errors.Newf("unsupported type %+v", reflect.TypeOf(typ.Typ))
	}

	return struc, nil
}

// GetConcreteType returns a concrete type for the given schema. This means any generic types
// in the top level type will be resolved to their concrete types and there will be no generic parameters in returned typ.
// However, any nested types may still contain generic types.
//
// If a nil schema is provided, a nil is returned.
func GetConcreteType(appDecls []*schema.Decl, originalType *schema.Type, typeArgs []*schema.Type) (*schema.Type, error) {
	if originalType == nil {
		// If there's no schema type, we want to shortcut
		return nil, nil
	}

	switch typ := originalType.Typ.(type) {
	case *schema.Type_Struct:
		// If there are no type arguments, we've got a concrete type
		if len(typeArgs) == 0 {
			return originalType, nil
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

		return &schema.Type{Typ: &schema.Type_Struct{Struct: struc}}, nil

	case *schema.Type_Map:
		// If there are no type arguments, we've got a concrete type
		if len(typeArgs) == 0 {
			return originalType, nil
		}

		// Deep copy the original struct
		mapType, ok := proto.Clone(typ.Map).(*schema.Map)
		if !ok {
			return nil, errors.New("failed to clone map")
		}

		return resolveTypeParams(&schema.Type{Typ: &schema.Type_Map{Map: mapType}}, typeArgs), nil

	case *schema.Type_List:
		// If there are no type arguments, we've got a concrete type
		if len(typeArgs) == 0 {
			return originalType, nil
		}

		// Deep copy the original struct
		list, ok := proto.Clone(typ.List).(*schema.List)
		if !ok {
			return nil, errors.New("failed to clone list type")
		}

		// replace any type parameters with the type argument
		return resolveTypeParams(&schema.Type{Typ: &schema.Type_List{List: list}}, typeArgs), nil

	case *schema.Type_Pointer:
		// If there are no type arguments, we've got a concrete type
		if len(typeArgs) == 0 {
			return originalType, nil
		}

		// Deep copy the original struct
		pointer, ok := proto.Clone(typ.Pointer).(*schema.Pointer)
		if !ok {
			return nil, errors.New("failed to clone pointer type")
		}

		var err error
		pointer.Base, err = GetConcreteType(appDecls, pointer.Base, typeArgs)
		if err != nil {
			return nil, err
		}

		// replace any type parameters with the type argument
		return resolveTypeParams(&schema.Type{Typ: &schema.Type_Pointer{Pointer: pointer}}, typeArgs), nil

	case *schema.Type_Config:
		// If there are no type arguments, we've got a concrete type
		if len(typeArgs) == 0 {
			return originalType, nil
		}

		// Deep copy the original struct
		config, ok := proto.Clone(typ.Config).(*schema.ConfigValue)
		if !ok {
			return nil, errors.New("failed to clone config type")
		}

		// replace any type parameters with the type argument
		return resolveTypeParams(&schema.Type{Typ: &schema.Type_Config{Config: config}}, typeArgs), nil

	case *schema.Type_Named:
		decl := appDecls[typ.Named.Id]
		return GetConcreteType(appDecls, decl.Type, typ.Named.TypeArguments)

	case *schema.Type_Builtin:
		return originalType, nil

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

	case *schema.Type_Struct:
		for _, field := range t.Struct.Fields {
			field.Typ = resolveTypeParams(field.Typ, typeArgs)
		}

	case *schema.Type_List:
		t.List.Elem = resolveTypeParams(t.List.Elem, typeArgs)

	case *schema.Type_Map:
		t.Map.Key = resolveTypeParams(t.Map.Key, typeArgs)
		t.Map.Value = resolveTypeParams(t.Map.Value, typeArgs)

	case *schema.Type_Config:
		t.Config.Elem = resolveTypeParams(t.Config.Elem, typeArgs)

	case *schema.Type_Pointer:
		t.Pointer.Base = resolveTypeParams(t.Pointer.Base, typeArgs)

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

// DescribeAuth generates a ParameterEncoding per field of the auth struct and returns it as
// the AuthEncoding. If authSchema is nil it returns nil, nil.
func DescribeAuth(appMetaData *meta.Data, authSchema *schema.Type, options *Options) (*AuthEncoding, error) {
	if authSchema == nil {
		return nil, nil
	}

	switch t := authSchema.Typ.(type) {
	case *schema.Type_Builtin:
		if t.Builtin != schema.Builtin_STRING {
			return nil, errors.Newf("unsupported auth parameter %v", errors.Safe(t.Builtin))
		}
		return &AuthEncoding{LegacyTokenFormat: true}, nil
	case *schema.Type_Named:
	case *schema.Type_Pointer:
	default:
		return nil, errors.Newf("unsupported auth parameter type %T", errors.Safe(t))
	}

	authStruct, err := GetConcreteStructType(appMetaData.Decls, authSchema, nil)
	if err != nil {
		return nil, errors.Wrap(err, "auth struct")
	}
	fields, err := describeParams(&encodingHints{Undefined, authTags, options}, authStruct)
	if err != nil {
		return nil, err
	}
	if locationDiff := keyDiff(fields, Header, Query, Cookie); len(locationDiff) > 0 {
		return nil, errors.Newf("auth must only contain query, header, and cookie parameters. Found: %v", locationDiff)
	}
	return &AuthEncoding{
		QueryParameters:  fields[Query],
		HeaderParameters: fields[Header],
		CookieParameters: fields[Cookie],
	}, nil
}

// DescribeResponse generates a ParameterEncoding per field of the response struct and returns it as
// the ResponseEncoding
func DescribeResponse(appMetaData *meta.Data, responseSchema *schema.Type, options *Options) (*ResponseEncoding, error) {
	if responseSchema == nil {
		return nil, nil
	}
	responseStruct, err := GetConcreteStructType(appMetaData.Decls, responseSchema, nil)
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
	for k := range src {
		if !slices.Contains(keys, k) {
			diff = append(diff, k)
		}
	}
	return diff
}

// DescribeRequest groups the provided httpMethods by default ParameterLocation and returns a RequestEncoding
// per ParameterLocation
func DescribeRequest(appMetaData *meta.Data, requestSchema *schema.Type, options *Options, httpMethods ...string) ([]*RequestEncoding, error) {
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

	var requestStruct *schema.Struct
	var err error
	if requestSchema != nil {
		requestStruct, err = GetConcreteStructType(appMetaData.Decls, requestSchema, nil)
		if err != nil {
			return nil, errors.Wrap(err, "request struct")
		}
	}

	var reqs []*RequestEncoding
	for location, methods := range methodsByDefaultLocation {
		var fields map[ParameterLocation][]*ParameterEncoding

		if requestStruct != nil {
			fields, err = describeParams(&encodingHints{location, requestTags, options}, requestStruct)
			if err != nil {
				return nil, err
			}
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
func formatName(location ParameterLocation, name string) string {
	switch location {
	case Query:
		return idents.Convert(name, idents.SnakeCase)
	default:
		return name
	}
}

// IgnoreField returns true if the field name is "-" is any of the valid request or response tags
func IgnoreField(field *schema.Field) bool {
	for _, tag := range field.Tags {
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
func describeParam(encodingHints *encodingHints, field *schema.Field) (*ParameterEncoding, error) {
	location := encodingHints.defaultLocation
	name := formatName(encodingHints.defaultLocation, field.Name)
	param := ParameterEncoding{
		Name:       name,
		OmitEmpty:  false,
		SrcName:    field.Name,
		Doc:        field.Doc,
		Type:       field.Typ,
		RawTag:     field.RawTag,
		Optional:   field.Optional,
		WireFormat: name,
	}

	var usedOverrideTag string
	for _, tag := range field.Tags {
		if IgnoreField(field) {
			return nil, nil
		}

		tagHint, ok := encodingHints.tags[tag.Key]
		if !ok {
			continue
		}

		if tagHint.overrideDefault {
			if usedOverrideTag != "" {
				return nil, errors.Newf("tag conflict: %s cannot be combined with %s", usedOverrideTag, tag.Key)
			}
			location = tagHint.location
			usedOverrideTag = tag.Key
		}
		if tagHint.location == location {
			param.Name = tag.Name
			if tagHint.wireFormatter != nil {
				param.WireFormat = tagHint.wireFormatter(tag.Name)
			} else {
				param.WireFormat = tag.Name
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

	if param.Name == "-" {
		return nil, nil
	}

	param.Location = location
	return &param, nil
}

// toEncodingMap returns a map from SrcName to parameter encodings.
func toEncodingMap(keyFunc func(e *ParameterEncoding) string, encodings ...[]*ParameterEncoding) map[string]*ParameterEncoding {
	res := make(map[string]*ParameterEncoding)
	for _, e := range encodings {
		for _, param := range e {
			res[keyFunc(param)] = param
		}
	}
	return res
}

// toEncodingMultiMap returns a map from a key to the list of parameter encodings
// matching that key.
func toEncodingMultiMap(keyFunc func(e *ParameterEncoding) string, encodings ...[]*ParameterEncoding) map[string][]*ParameterEncoding {
	res := make(map[string][]*ParameterEncoding)
	for _, e := range encodings {
		for _, param := range e {
			key := keyFunc(param)
			res[key] = append(res[key], param)
		}
	}
	return res
}

func srcNameKey(e *ParameterEncoding) string {
	return e.SrcName
}

func nameKey(e *ParameterEncoding) string {
	return e.Name
}
