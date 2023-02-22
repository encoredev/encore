package rpc

import (
	"errors"
	"fmt"
	"go/ast"
	"strings"

	"golang.org/x/exp/slices"

	"encr.dev/parser2/apis/apipaths"
	"encr.dev/parser2/apis/directive"
	"encr.dev/parser2/apis/selector"
	"encr.dev/parser2/internal/perr"
	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
	"encr.dev/parser2/internal/schema/schemautil"
	"encr.dev/pkg/option"
)

type AccessType string

const (
	Public  AccessType = "public"
	Private AccessType = "private"
	// Auth is like public but requires authentication.
	Auth AccessType = "auth"
)

type RPC struct {
	Name        string
	Doc         string
	File        *pkginfo.File
	Decl        *schema.FuncDecl
	Access      AccessType
	Raw         bool
	Path        apipaths.Path
	HTTPMethods []string
	Request     schema.Type // request data; nil for Raw RPCs
	Response    schema.Type // response data; nil for Raw RPCs
	Tags        selector.Set
	Recv        option.Option[*schema.Receiver] // None if not a method
}

type ParseData struct {
	Errs   *perr.List
	Schema *schema.Parser

	File *pkginfo.File
	Func *ast.FuncDecl
	Dir  directive.Directive
	Doc  string
}

// Parse parses an RPC endpoint. It may return nil on errors.
func Parse(d ParseData) *RPC {
	rpc, err := validateDirective(d.Dir)
	if err != nil {
		d.Errs.Addf(d.Dir.AST.Pos(), "invalid encore:%s directive: %v", d.Dir.Name, err)
		return nil
	}

	// If there was no path, default to "pkg.Decl".
	if rpc.Path.Segments == nil {
		rpc.Path = apipaths.Path{
			Pos: d.Dir.AST.Pos(),
			Segments: []apipaths.Segment{{
				Type:      apipaths.Literal,
				Value:     d.File.Pkg.Name + "." + d.Func.Name.Name,
				ValueType: schema.String,
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
	// TODO(andre) Replace this with the RPC encoding.
	if len(rpc.HTTPMethods) == 0 {
		if rpc.Raw {
			rpc.HTTPMethods = []string{"*"}
		} else {
			// For non-raw endpoints, if there's a request payload
			// default to POST-only.
			// TODO(andre) base this on the RPC encoding!
			if rpc.Request != nil {
				rpc.HTTPMethods = []string{"POST"}
			} else {
				rpc.HTTPMethods = []string{"GET", "POST"}
			}
		}
	}

	// Validate the RPC.
	if rpc.Raw {
		validateRawRPC(d.Errs, rpc)
	} else {
		initTypedRPC(d.Errs, rpc)
	}

	return rpc
}

func initTypedRPC(errs *perr.List, rpc *RPC) {
	const sigHint = `
	hint: valid signatures are:
	- func(context.Context) error
	- func(context.Context) (*ResponseData, error)
	- func(context.Context, *RequestData) error
	- func(context.Context, *RequestType) (*ResponseData, error)`

	decl := rpc.Decl
	sig := decl.Type
	numParams := len(sig.Params)
	if numParams == 0 {
		errs.Add(sig.AST.Pos(), "invalid API signature (too few parameters)"+sigHint)
		return
	}

	numResults := len(sig.Results)
	if numResults == 0 {
		errs.Add(sig.AST.Pos(), "invalid API signature (too few results)"+sigHint)
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
	for i := 0; i < len(rpc.Path.Segments); i++ {
		if s := &rpc.Path.Segments[i]; s.Type != apipaths.Literal {
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
			rpc.Request = param.Type
		}
	}

	if seenParams < len(pathParams) {
		var missing []string
		for i := seenParams; i < len(pathParams); i++ {
			missing = append(missing, pathParams[i].Value)
		}
		errs.Addf(sig.AST.Pos(), "invalid API signature: expected function parameters named '%s' to match API path params",
			strings.Join(missing, "', '"))
	}

	// First return value must be *T or *pkg.T
	if numResults >= 2 {
		result := sig.Results[0]
		rpc.Response = result.Type
		if numResults > 2 {
			errs.Add(sig.Results[2].AST.Pos(), "API signature cannot contain more than two results"+sigHint)
			return
		}
	}

	// Make sure the last return is of type error.
	if err := sig.Results[numResults-1]; !schemautil.IsBuiltinKind(err.Type, schema.Error) {
		errs.Add(err.AST.Pos(), "last result is not of type error"+sigHint)
		return
	}
}

func validateRawRPC(errs *perr.List, rpc *RPC) {
	const sigHint = `
	hint: signature must be func(http.ResponseWriter, *http.Request)`

	decl := rpc.Decl
	sig := decl.Type
	params := sig.Params
	if len(params) < 2 {
		errs.Add(sig.AST.Pos(), "invalid API signature (too few parameters)"+sigHint)
		return
	} else if len(params) > 2 {
		errs.Add(params[2].AST.Pos(), "invalid API signature (too many parameters)"+sigHint)
		return
	} else if len(sig.Results) > 0 {
		errs.Addf(sig.Results[0].AST.Pos(), "invalid API signature (too many results)"+sigHint)
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
func validatePathParam(errs *perr.List, param schema.Param, seg *apipaths.Segment) schema.BuiltinKind {
	if param.Name.Value != seg.Value {
		errs.Addf(param.AST.Pos(), "unexpected parameter name '%s', expected '%s' (to match path parameter '%s')",
			param.Name.Value, seg.Value, seg.String())
	}

	builtin, _ := param.Type.(schema.BuiltinType)
	b := builtin.Kind

	// Wildcard path parameters must be strings.
	if seg.Type == apipaths.Wildcard && b != schema.String {
		errs.Addf(param.AST.Pos(), "wildcard path parameter '%s' must be a string", param.Name)
	}

	switch b {
	case schema.String, schema.Bool,
		schema.Int, schema.Int8, schema.Int16, schema.Int32, schema.Int64,
		schema.Uint, schema.Uint8, schema.Uint16, schema.Uint32, schema.Uint64,
		schema.UUID:
		return b
	default:
		errs.Addf(param.AST.Pos(), "path parameter '%s' must be a string, bool, integer, or encore.dev/types/uuid.UUID", param.Name)
		return schema.Invalid
	}
}

// validateDirective validates the given encore:api directive
// and returns an RPC with the respective fields set.
func validateDirective(dir directive.Directive) (*RPC, error) {
	rpc := &RPC{
		Raw: dir.HasOption("raw"),
	}

	accessOptions := []string{"public", "private", "auth"}
	err := directive.Validate(dir, directive.ValidateSpec{
		AllowedOptions: append([]string{"raw"}, accessOptions...),
		AllowedFields:  []string{"path", "method"},

		ValidateOption: func(opt string) error {
			// If this is an access option, check for duplicates.
			if slices.Contains(accessOptions, opt) {
				if rpc.Access != "" {
					return fmt.Errorf("duplicate access options: %s and %s", rpc.Access, opt)
				}
				rpc.Access = AccessType(opt)
			}

			return nil
		},
		ValidateField: func(f directive.Field) (err error) {
			switch f.Key {
			case "path":
				rpc.Path, err = apipaths.Parse(dir.AST.Pos(), f.Value)

			case "method":
				rpc.HTTPMethods = f.List()
				for _, m := range rpc.HTTPMethods {
					for _, c := range m {
						if !(c >= 'A' && c <= 'Z') && !(c >= 'a' && c <= 'z') {
							return fmt.Errorf("invalid API method: %q", m)
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
			rpc.Tags.Add(sel)
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("invalid encore:api directive: %v", err)
	}

	// Access defaults to private if not provided.
	if rpc.Access == "" {
		rpc.Access = Private
	}
	if rpc.Access == Private && rpc.Raw {
		// We don't support private raw APIs for now.
		return nil, fmt.Errorf("invalid encore:api directive: private APIs cannot be declared raw")
	}

	return rpc, nil
}
