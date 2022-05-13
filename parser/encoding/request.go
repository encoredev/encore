package encoding

import (
	"github.com/cockroachdb/errors"

	"encr.dev/parser"
	v1 "encr.dev/proto/encore/parser/schema/v1"
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
	Field *v1.Field
}

// DescribeRequest combines a payload v1.Struct and a list of HTTP methods and returns
// a list of RPCRequest metadata. RPCRequest specifies which of the requested HTTP method
// it can be used for.
func DescribeRequest(payload *v1.Struct, httpMethods ...string) ([]*RequestEncoding, error) {
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
func describeParams(defaultLocation ParameterLocation, payload *v1.Struct) (fields []*ParameterEncoding, err error) {
	for _, f := range payload.GetFields() {
		f, err := describeParam(defaultLocation, f)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
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
func describeParam(defaultLocation ParameterLocation, field *v1.Field) (*ParameterEncoding, error) {
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
