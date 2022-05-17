package encoding

import (
	"reflect"

	"github.com/cockroachdb/errors"
	"github.com/golang/protobuf/proto"

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

// tagToLocation is used to translate struct field tags to param locations
// if overrideDefault is set, the default HTTP param location will be changed
// if the tag matches the paramLocation, the param name will be override with the
// tag name
var tagToLocation = map[string]struct {
	location        ParameterLocation
	overrideDefault bool
}{
	"query": {
		location:        Query,
		overrideDefault: true,
	},
	"qs": {
		location:        Query,
		overrideDefault: true,
	},
	"header": {
		location:        Header,
		overrideDefault: true,
	},
	"json": {
		location:        Body,
		overrideDefault: false,
	},
}

// RPCEncoding expresses how an RPC should be encoded on the wire for both the request and responses.
type RPCEncoding struct {
	// Expresses how the default request encoding and method should be
	// Note: DefaultRequestEncoding.HTTPMethods will always be a slice with length 1
	DefaultRequestEncoding *RequestEncoding
	// Expresses all the different ways the request can be encoded for this RPC
	RequestEncoding []*RequestEncoding
	// Expresses how the response to this RPC will be encoded
	ResponseEncoding []*ParameterEncoding
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

// RequestEncoding expresses how a request should be encoded for a specified set of HTTPMethods
type RequestEncoding struct {
	// The HTTP methods these field configurations can be used for
	HTTPMethods []string
	// Contains metadata about how to marshal a HTTP parameter
	Fields []*ParameterEncoding
}

// ParameterEncoding expresses how a parameter should be encoded
type ParameterEncoding struct {
	// Location (e.g. header, query, body) where the parameter should be marshalled from/to
	Location ParameterLocation
	// The location specific name of the parameter (e.g. cheeseEater, cheese-eater, X-Cheese-Eater
	Name string
	// The underlying field definition
	Field *schema.Field
}

// DescribeRPC expresses how to encode an RPCs request and response objects for the wire.
func DescribeRPC(appMetaData *meta.Data, rpc *meta.RPC) (*RPCEncoding, error) {
	encoding := &RPCEncoding{}
	// Work out the request encoding
	requestStruct, err := getConcreteStructType(appMetaData, rpc.RequestSchema, nil)
	if err != nil {
		return nil, errors.Wrap(err, "request struct")
	}
	encoding.RequestEncoding, err = DescribeRequest(requestStruct, rpc.HttpMethods...)
	if err != nil {
		return nil, errors.Wrap(err, "request encoding")
	}

	// Work out the response encoding
	_, err = getConcreteStructType(appMetaData, rpc.ResponseSchema, nil)
	if err != nil {
		return nil, errors.Wrap(err, "response struct")
	}
	// TODO(stefan): implement response encoding here

	// Setup the default request encoding
	defaultMethod := DefaultClientHttpMethod(rpc)
	defaultEncoding := encoding.RequestEncodingForMethod(defaultMethod)
	encoding.DefaultRequestEncoding = &RequestEncoding{
		HTTPMethods: []string{defaultMethod},
		Fields:      defaultEncoding.Fields,
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
			if typeParam := field.Typ.GetTypeParameter(); typeParam != nil {
				field.Typ = typeArgs[typeParam.ParamIdx]
			}
		}

		return struc, nil
	case *schema.Type_Named:
		decl := appMetaData.Decls[typ.Named.Id]
		return getConcreteStructType(appMetaData, decl.Type, typ.Named.TypeArguments)
	default:
		return nil, errors.Newf("unsupported type %+v", reflect.TypeOf(typ))
	}
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

// DescribeRequest combines a payload schema.Struct and a list of HTTP methods and returns
// a list of RPCRequest metadata. RPCRequest specifies which of the requested HTTP method
// it can be used for.
func DescribeRequest(payload *schema.Struct, httpMethods ...string) ([]*RequestEncoding, error) {
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
		fields, err := describeParams(location, payload)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, &RequestEncoding{
			HTTPMethods: methods,
			Fields:      fields,
		})
	}
	return reqs, nil
}

// describeParams calls describeParam() for each field in the payload struct
func describeParams(defaultLocation ParameterLocation, payload *schema.Struct) (fields []*ParameterEncoding, err error) {
	for _, f := range payload.GetFields() {
		f, err := describeParam(defaultLocation, f)
		if err != nil {
			return nil, err
		}

		// fields explicitly named "-" are excluded from the generated client
		if f.Name != "-" {
			fields = append(fields, f)
		}
	}
	return fields, nil
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

// describeParam returns an RPCField which falls back on defaultLocation if no parameter
// (e.g. qs, query, header) tag is set
func describeParam(defaultLocation ParameterLocation, field *schema.Field) (*ParameterEncoding, error) {
	rpcField := &ParameterEncoding{defaultLocation, formatName(defaultLocation, field.Name), field}
	var usedOverrideTag string
	for _, tag := range field.Tags {
		spec, ok := tagToLocation[tag.Key]
		if !ok {
			continue
		}
		if spec.overrideDefault {
			if usedOverrideTag != "" {
				return nil, errors.Newf("tag conflict: %s cannot be combined with %s", usedOverrideTag, tag.Key)
			}
			rpcField.Location = spec.location
			usedOverrideTag = tag.Key
		}
		if spec.location == rpcField.Location {
			rpcField.Name = tag.Name
		}
	}
	return rpcField, nil
}
