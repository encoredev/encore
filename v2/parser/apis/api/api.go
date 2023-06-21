package api

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
	"sync"

	"golang.org/x/exp/slices"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/option"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/resourcepaths"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/apis/api/apienc"
	"encr.dev/v2/parser/apis/internal/directive"
	"encr.dev/v2/parser/apis/selector"
	"encr.dev/v2/parser/resource"
)

type AccessType string

const (
	Public  AccessType = "public"
	Private AccessType = "private"
	// Auth is like public but requires authentication.
	Auth AccessType = "auth"
)

type Endpoint struct {
	errs *perr.List

	Name             string
	Doc              string
	File             *pkginfo.File
	Decl             *schema.FuncDecl
	Access           AccessType
	AccessField      option.Option[directive.Field]
	Raw              bool
	Path             *resourcepaths.Path
	HTTPMethods      []string
	HTTPMethodsField option.Option[directive.Field]
	Request          schema.Type // request data; nil for Raw Endpoints
	Response         schema.Type // response data; nil for Raw Endpoints
	Tags             selector.Set
	Recv             option.Option[*schema.Receiver] // None if not a method

	reqEncOnce  sync.Once
	reqEncoding []*apienc.RequestEncoding

	respEncOnce  sync.Once
	respEncoding *apienc.ResponseEncoding
}

func (ep *Endpoint) GoString() string {
	if ep == nil {
		return "(*api.Endpoint)(nil)"
	}
	return fmt.Sprintf("&api.Endpoint{Name: %q}", ep.Name)
}

func (ep *Endpoint) Kind() resource.Kind       { return resource.APIEndpoint }
func (ep *Endpoint) Package() *pkginfo.Package { return ep.File.Pkg }
func (ep *Endpoint) Pos() token.Pos            { return ep.Decl.AST.Pos() }
func (ep *Endpoint) End() token.Pos            { return ep.Decl.AST.End() }
func (ep *Endpoint) SortKey() string           { return ep.File.Pkg.ImportPath.String() + "." + ep.Name }

func (ep *Endpoint) RequestEncoding() []*apienc.RequestEncoding {
	if ep.Request == nil {
		return nil
	}

	ep.reqEncOnce.Do(func() {
		requestParam := ep.Decl.Type.Params[len(ep.Decl.Type.Params)-1]
		ep.reqEncoding = apienc.DescribeRequest(ep.errs, requestParam, ep.Request, ep.HTTPMethodsField, ep.HTTPMethods...)
	})
	return ep.reqEncoding
}

func (ep *Endpoint) ResponseEncoding() *apienc.ResponseEncoding {
	ep.respEncOnce.Do(func() {
		ep.respEncoding = apienc.DescribeResponse(ep.errs, ep.Response)
	})
	return ep.respEncoding
}

type ParseData struct {
	Errs   *perr.List
	Schema *schema.Parser

	File *pkginfo.File
	Func *ast.FuncDecl
	Dir  *directive.Directive
	Doc  string
}

// Parse parses an API endpoint. It may return nil on errors.
func Parse(d ParseData) *Endpoint {
	rpc, ok := validateDirective(d.Errs, d.Dir)
	if !ok {
		return nil
	}
	rpc.errs = d.Errs

	// If there was no path, default to "pkg.Decl".
	if rpc.Path == nil {
		rpc.Path = &resourcepaths.Path{
			StartPos: d.Func.Name.Pos(),
			Segments: []resourcepaths.Segment{{
				Type:      resourcepaths.Literal,
				Value:     d.File.Pkg.Name + "." + d.Func.Name.Name,
				ValueType: schema.String,
				StartPos:  d.Func.Name.Pos(),
				EndPos:    d.Func.Name.End(),
			}},
		}
	}

	decl, ok := d.Schema.ParseFuncDecl(d.File, d.Func)
	if !ok {
		return nil
	}

	rpc.Name = d.Func.Name.Name
	rpc.Doc = d.Doc
	rpc.Decl = decl
	rpc.File = d.File
	rpc.Recv = decl.Recv

	// Validate the API.
	if rpc.Raw {
		initRawRPC(d.Errs, rpc)
	} else {
		initTypedRPC(d.Errs, rpc)
	}

	// If we didn't get any HTTP methods, set a reasonable default.
	// TODO(andre) Replace this with the API encoding.
	if len(rpc.HTTPMethods) == 0 {
		if rpc.Raw {
			rpc.HTTPMethods = []string{"*"}
		} else {
			// For non-raw endpoints, if there's a request payload
			// default to POST-only.
			if rpc.Request != nil {
				rpc.HTTPMethods = []string{"POST"}
			} else {
				rpc.HTTPMethods = []string{"GET", "POST"}
			}
		}
	}

	return rpc
}

func initTypedRPC(errs *perr.List, endpoint *Endpoint) {
	decl := endpoint.Decl
	sig := decl.Type
	numParams := len(sig.Params)
	if numParams == 0 {
		errs.Add(errWrongNumberParams(numParams).AtGoNode(sig.AST.Params))
		return
	}

	numResults := len(sig.Results)
	if numResults == 0 || numResults > 2 {
		errs.Add(errWrongNumberResults(numResults).AtGoNode(sig.AST.Results))
		return
	}

	// First type should always be context.Context
	ctxParam := sig.Params[0]
	if !schemautil.IsNamed(ctxParam.Type, "context", "Context") {
		errs.Add(errInvalidFirstParam.AtGoNode(ctxParam.AST))
		return
	}

	// For each path parameter, expect a parameter to match it
	var pathParams []*resourcepaths.Segment
	for i := 0; i < len(endpoint.Path.Segments); i++ {
		if s := &endpoint.Path.Segments[i]; s.Type != resourcepaths.Literal {
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
				errs.Add(errMultiplePayloads.AtGoNode(param.AST))
				continue
			}
			endpoint.Request = param.Type
		}
	}

	if seenParams < len(pathParams) {
		var missing []string
		var missingParams []*resourcepaths.Segment
		for i := seenParams; i < len(pathParams); i++ {
			missing = append(missing, pathParams[i].Value)
			missingParams = append(missingParams, pathParams[i])
		}

		err := errInvalidPathParams(strings.Join(missing, "', '")).AtGoNode(sig.AST.Params)
		for _, p := range missingParams {
			err = err.AtGoNode(p)
		}

		errs.Add(err)
	}

	// First return value must be *T or *pkg.T
	if numResults >= 2 {
		result := sig.Results[0]
		endpoint.Response = result.Type
	}

	// Make sure the last return is of type error.
	if err := sig.Results[numResults-1]; !schemautil.IsBuiltinKind(err.Type, schema.Error) {
		errs.Add(errLastResultMustBeError.AtGoNode(err.AST))
		return
	}
}

func initRawRPC(errs *perr.List, endpoint *Endpoint) {
	decl := endpoint.Decl
	sig := decl.Type
	params := sig.Params
	if len(params) < 2 {
		errs.Add(errInvalidRawParams(len(params)).AtGoNode(sig.AST.Params))
		return
	} else if len(params) > 2 {
		errs.Add(errInvalidRawParams(len(params)).AtGoNode(sig.AST.Params))
	} else if len(sig.Results) > 0 {
		errs.Add(errInvalidRawResults(len(sig.Results)).AtGoNode(sig.AST.Results))
		return
	}

	// Ensure signature is func(http.ResponseWriter, *http.Request).
	if !schemautil.IsNamed(params[0].Type, "net/http", "ResponseWriter") {
		errs.Add(errRawNotResponeWriter.AtGoNode(params[0].AST))
	}
	if deref, n := schemautil.Deref(params[1].Type); n != 1 || !schemautil.IsNamed(deref, "net/http", "Request") {
		errs.Add(errRawNotRequest.AtGoNode(params[1].AST))
	}
}

// validatePathParam validates that the given func parameter is compatible with the given path segment.
// It checks that the names match and that the func parameter is of a permissible type.
// It returns the func parameter's builtin kind.
func validatePathParam(errs *perr.List, param schema.Param, seg *resourcepaths.Segment) schema.BuiltinKind {
	if !option.Contains(param.Name, seg.Value) {
		errs.Add(errUnexpectedParameterName(param.Name, seg.Value, seg.String()).AtGoNode(seg).AtGoNode(param.AST))
	}

	builtin, _ := param.Type.(schema.BuiltinType)
	b := builtin.Kind

	// Wildcard path parameters must be strings.
	if b != schema.String && (seg.Type == resourcepaths.Wildcard || seg.Type == resourcepaths.Fallback) {
		errs.Add(errWildcardMustBeString(param.Name).AtGoNode(seg).AtGoNode(param.AST))
	}

	switch b {
	case schema.String, schema.Bool,
		schema.Int, schema.Int8, schema.Int16, schema.Int32, schema.Int64,
		schema.Uint, schema.Uint8, schema.Uint16, schema.Uint32, schema.Uint64,
		schema.UUID:
		return b
	default:
		errs.Add(errInvalidPathParamType(param.Name).AtGoNode(seg).AtGoNode(param.AST))
		return schema.Invalid
	}
}

// validateDirective validates the given encore:api directive
// and returns an API with the respective fields set.
func validateDirective(errs *perr.List, dir *directive.Directive) (*Endpoint, bool) {
	endpoint := &Endpoint{
		Raw: dir.HasOption("raw"),
	}

	var accessField directive.Field
	var rawTag directive.Field

	accessOptions := []string{"public", "private", "auth"}
	ok := directive.Validate(errs, dir, directive.ValidateSpec{
		AllowedOptions: append([]string{"raw"}, accessOptions...),
		AllowedFields:  []string{"path", "method"},

		ValidateOption: func(errs *perr.List, opt directive.Field) (ok bool) {
			// If this is an access option, check for duplicates.
			if slices.Contains(accessOptions, opt.Value) {
				if endpoint.Access != "" {
					errs.Add(errDuplicateAccessOptions(endpoint.Access, opt.Value, strings.Join(accessOptions, ", ")).AtGoNode(opt).AtGoNode(accessField))
					return false
				}
				accessField = opt
				endpoint.Access = AccessType(opt.Value)
				endpoint.AccessField = option.Some(opt)
			}

			if opt.Value == "raw" {
				rawTag = opt
			}

			return true
		},
		ValidateField: func(errs *perr.List, f directive.Field) (ok bool) {
			switch f.Key {
			case "path":
				endpoint.Path, ok = resourcepaths.Parse(
					errs,
					f.End()-token.Pos(len([]byte(f.Value))-1),
					f.Value,
					resourcepaths.Options{
						AllowWildcard: true,
						AllowFallback: true,
						PrefixSlash:   true,
					},
				)
				if !ok {
					return false
				}

			case "method":
				endpoint.HTTPMethods = f.List()
				endpoint.HTTPMethodsField = option.Some(f)
				for _, m := range endpoint.HTTPMethods {
					for _, c := range m {
						if !(c >= 'A' && c <= 'Z') && !(c >= 'a' && c <= 'z') {
							errs.Add(errInvalidEndpointMethod(m).AtGoNode(f))
							return false
						} else if !(c >= 'A' && c <= 'Z') {
							errs.Add(errEndpointMethodMustBeAllCaps.AtGoNode(f))
							return false
						}
					}
				}
			}
			return true
		},
		ValidateTag: func(errs *perr.List, tag directive.Field) (ok bool) {
			sel, ok := selector.Parse(errs, tag.Pos(), tag.Value)
			if !ok {
				return false
			}
			endpoint.Tags.Add(sel)
			return true
		},
	})
	if !ok {
		return nil, false
	}

	// Access defaults to private if not provided.
	if endpoint.Access == "" {
		endpoint.Access = Private
	}
	if endpoint.Access == Private && endpoint.Raw {
		// We don't support private raw APIs for now.
		errs.Add(errRawEndpointCantBePrivate.AtGoNode(rawTag, errors.AsError("declared as raw here")).AtGoNode(accessField, errors.AsError("set as private here")))
		return nil, false
	}

	return endpoint, true
}
