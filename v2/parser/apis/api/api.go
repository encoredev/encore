package api

import (
	"errors"
	"fmt"
	"go/ast"
	"strings"

	"golang.org/x/exp/slices"

	"encr.dev/pkg/option"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	schema2 "encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	"encr.dev/v2/parser/apis/api/apipaths"
	"encr.dev/v2/parser/apis/directive"
	"encr.dev/v2/parser/apis/selector"
)

type AccessType string

const (
	Public  AccessType = "public"
	Private AccessType = "private"
	// Auth is like public but requires authentication.
	Auth AccessType = "auth"
)

type Endpoint struct {
	Name        string
	Doc         string
	File        *pkginfo.File
	Decl        *schema2.FuncDecl
	Access      AccessType
	Raw         bool
	Path        *apipaths.Path
	HTTPMethods []string
	Request     schema2.Type // request data; nil for Raw Endpoints
	Response    schema2.Type // response data; nil for Raw Endpoints
	Tags        selector.Set
	Recv        option.Option[*schema2.Receiver] // None if not a method
}

type ParseData struct {
	Errs   *perr.List
	Schema *schema2.Parser

	File *pkginfo.File
	Func *ast.FuncDecl
	Dir  *directive.Directive
	Doc  string
}

// Parse parses an API endpoint. It may return nil on errors.
func Parse(d ParseData) *Endpoint {
	rpc, err := validateDirective(d.Dir)
	if err != nil {
		d.Errs.Addf(d.Dir.AST.Pos(), "invalid encore:%s directive: %v", d.Dir.Name, err)
		return nil
	}

	// If there was no path, default to "pkg.Decl".
	if rpc.Path == nil {
		rpc.Path = &apipaths.Path{
			Pos: d.Dir.AST.Pos(),
			Segments: []apipaths.Segment{{
				Type:      apipaths.Literal,
				Value:     d.File.Pkg.Name + "." + d.Func.Name.Name,
				ValueType: schema2.String,
			}},
		}
	}

	decl := d.Schema.ParseFuncDecl(d.File, d.Func)

	rpc.Name = d.Func.Name.Name
	rpc.Doc = d.Doc
	rpc.Decl = decl
	rpc.File = d.File
	rpc.Recv = decl.Recv

	// If we didn't get any HTTP methods, set a reasonable default.
	// TODO(andre) Replace this with the API encoding.
	if len(rpc.HTTPMethods) == 0 {
		if rpc.Raw {
			rpc.HTTPMethods = []string{"*"}
		} else {
			// For non-raw endpoints, if there's a request payload
			// default to POST-only.
			// TODO(andre) base this on the API encoding!
			if rpc.Request != nil {
				rpc.HTTPMethods = []string{"POST"}
			} else {
				rpc.HTTPMethods = []string{"GET", "POST"}
			}
		}
	}

	// Validate the API.
	if rpc.Raw {
		validateRawRPC(d.Errs, rpc)
	} else {
		initTypedRPC(d.Errs, rpc)
	}

	return rpc
}

func initTypedRPC(errs *perr.List, endpoint *Endpoint) {
	const sigHint = `
	hint: valid signatures are:
	- func(context.Context) error
	- func(context.Context) (*ResponseData, error)
	- func(context.Context, *RequestData) error
	- func(context.Context, *RequestType) (*ResponseData, error)`

	decl := endpoint.Decl
	sig := decl.Type
	numParams := len(sig.Params)
	if numParams == 0 {
		errs.Add(sig.AST.Pos(), "invalid Endpoint signature (too few parameters)"+sigHint)
		return
	}

	numResults := len(sig.Results)
	if numResults == 0 {
		errs.Add(sig.AST.Pos(), "invalid Endpoint signature (too few results)"+sigHint)
		return
	}

	// First type should always be context.Context
	ctxParam := sig.Params[0]
	if !schemautil.IsNamed(ctxParam.Type, "context", "Context") {
		errs.Add(ctxParam.AST.Pos(), "first parameter must be of type context.Context"+sigHint)
		return
	}

	// For each path parameter, expect a parameter to match it
	var pathParams []*apipaths.Segment
	for i := 0; i < len(endpoint.Path.Segments); i++ {
		if s := &endpoint.Path.Segments[i]; s.Type != apipaths.Literal {
			pathParams = append(pathParams, s)
		}
	}

	seenParams := 0
	for i := 0; i < numParams-1; i++ {
		param := sig.Params[i+1] // +1 to skip context.Context

		// Is it a path parameter?
		if i < len(pathParams) {
			seg := pathParams[i]
			b := validatePathParam(errs, param, seg)
			pathParams[seenParams].ValueType = b
			seenParams++
		} else {
			// Otherwise it must be a payload parameter
			payloadIdx := i - len(pathParams)
			if payloadIdx > 0 {
				errs.Add(param.AST.Pos(), "APIs cannot have multiple payload parameters")
				continue
			}
			endpoint.Request = param.Type
		}
	}

	if seenParams < len(pathParams) {
		var missing []string
		for i := seenParams; i < len(pathParams); i++ {
			missing = append(missing, pathParams[i].Value)
		}
		errs.Addf(sig.AST.Pos(), "invalid Endpoint signature: expected function parameters named '%s' to match Endpoint path params",
			strings.Join(missing, "', '"))
	}

	// First return value must be *T or *pkg.T
	if numResults >= 2 {
		result := sig.Results[0]
		endpoint.Response = result.Type
		if numResults > 2 {
			errs.Add(sig.Results[2].AST.Pos(), "Endpoint signature cannot contain more than two results"+sigHint)
			return
		}
	}

	// Make sure the last return is of type error.
	if err := sig.Results[numResults-1]; !schemautil.IsBuiltinKind(err.Type, schema2.Error) {
		errs.Add(err.AST.Pos(), "last result is not of type error"+sigHint)
		return
	}
}

func validateRawRPC(errs *perr.List, endpoint *Endpoint) {
	const sigHint = `
	hint: signature must be func(http.ResponseWriter, *http.Request)`

	decl := endpoint.Decl
	sig := decl.Type
	params := sig.Params
	if len(params) < 2 {
		errs.Add(sig.AST.Pos(), "invalid Endpoint signature (too few parameters)"+sigHint)
		return
	} else if len(params) > 2 {
		errs.Add(params[2].AST.Pos(), "invalid Endpoint signature (too many parameters)"+sigHint)
		return
	} else if len(sig.Results) > 0 {
		errs.Addf(sig.Results[0].AST.Pos(), "invalid Endpoint signature (too many results)"+sigHint)
		return
	}

	// Ensure signature is func(http.ResponseWriter, *http.Request).
	if !schemautil.IsNamed(params[0].Type, "net/http", "ResponseWriter") {
		errs.Add(params[0].AST.Pos(), "first parameter must be http.ResponseWriter"+sigHint)
	}
	if deref, n := schemautil.Deref(params[1].Type); n != 1 || !schemautil.IsNamed(deref, "net/http", "Request") {
		errs.Add(params[1].AST.Pos(), "second parameter must be *http.Request"+sigHint)
	}
}

// validatePathParam validates that the given func parameter is compatible with the given path segment.
// It checks that the names match and that the func parameter is of a permissible type.
// It returns the func parameter's builtin kind.
func validatePathParam(errs *perr.List, param schema2.Param, seg *apipaths.Segment) schema2.BuiltinKind {
	if param.Name.Value != seg.Value {
		errs.Addf(param.AST.Pos(), "unexpected parameter name '%s', expected '%s' (to match path parameter '%s')",
			param.Name.Value, seg.Value, seg.String())
	}

	builtin, _ := param.Type.(schema2.BuiltinType)
	b := builtin.Kind

	// Wildcard path parameters must be strings.
	if seg.Type == apipaths.Wildcard && b != schema2.String {
		errs.Addf(param.AST.Pos(), "wildcard path parameter '%s' must be a string", param.Name)
	}

	switch b {
	case schema2.String, schema2.Bool,
		schema2.Int, schema2.Int8, schema2.Int16, schema2.Int32, schema2.Int64,
		schema2.Uint, schema2.Uint8, schema2.Uint16, schema2.Uint32, schema2.Uint64,
		schema2.UUID:
		return b
	default:
		errs.Addf(param.AST.Pos(), "path parameter '%s' must be a string, bool, integer, or encore.dev/types/uuid.UUID", param.Name)
		return schema2.Invalid
	}
}

// validateDirective validates the given encore:api directive
// and returns an API with the respective fields set.
func validateDirective(dir *directive.Directive) (*Endpoint, error) {
	endpoint := &Endpoint{
		Raw: dir.HasOption("raw"),
	}

	accessOptions := []string{"public", "private", "auth"}
	err := directive.Validate(dir, directive.ValidateSpec{
		AllowedOptions: append([]string{"raw"}, accessOptions...),
		AllowedFields:  []string{"path", "method"},

		ValidateOption: func(opt string) error {
			// If this is an access option, check for duplicates.
			if slices.Contains(accessOptions, opt) {
				if endpoint.Access != "" {
					return fmt.Errorf("duplicate access options: %s and %s", endpoint.Access, opt)
				}
				endpoint.Access = AccessType(opt)
			}

			return nil
		},
		ValidateField: func(f directive.Field) (err error) {
			switch f.Key {
			case "path":
				endpoint.Path, err = apipaths.Parse(dir.AST.Pos(), f.Value)

			case "method":
				endpoint.HTTPMethods = f.List()
				for _, m := range endpoint.HTTPMethods {
					for _, c := range m {
						if !(c >= 'A' && c <= 'Z') && !(c >= 'a' && c <= 'z') {
							return fmt.Errorf("invalid Endpoint method: %q", m)
						} else if !(c >= 'A' && c <= 'Z') {
							return errors.New("methods must be ALLCAPS")
						}
					}
				}
			}
			return err
		},
		ValidateTag: func(tag string) error {
			sel, err := selector.Parse(tag)
			if err != nil {
				return err
			}
			endpoint.Tags.Add(sel)
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("invalid encore:api directive: %v", err)
	}

	// Access defaults to private if not provided.
	if endpoint.Access == "" {
		endpoint.Access = Private
	}
	if endpoint.Access == Private && endpoint.Raw {
		// We don't support private raw APIs for now.
		return nil, fmt.Errorf("invalid encore:api directive: private APIs cannot be declared raw")
	}

	return endpoint, nil
}
