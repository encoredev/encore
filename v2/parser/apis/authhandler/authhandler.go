package authhandler

import (
	"go/ast"
	"go/token"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/option"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/apis/internal/directive"
	"encr.dev/v2/parser/resource"
)

// AuthHandler describes an Encore authentication handler.
type AuthHandler struct {
	Decl *schema.FuncDecl
	Doc  string
	Name string // the name of the auth handler.

	// Param is the auth parameters.
	// It's either a builtin string for token-based authentication,
	// or a named struct type for complex auth parameters.
	Param schema.Type

	// Recv is the type the auth handler is defined as a method on, if any.
	Recv option.Option[*schema.Receiver]

	// AuthData is the custom auth data type the app specifies
	// as part of the returns from the auth handler, if any.
	AuthData option.Option[*schema.TypeDeclRef]
}

func (ah *AuthHandler) Kind() resource.Kind       { return resource.AuthHandler }
func (ah *AuthHandler) Package() *pkginfo.Package { return ah.Decl.File.Pkg }
func (ah *AuthHandler) Pos() token.Pos            { return ah.Decl.AST.Pos() }
func (ah *AuthHandler) End() token.Pos            { return ah.Decl.AST.End() }
func (ah *AuthHandler) SortKey() string           { return ah.Decl.File.Pkg.ImportPath.String() + "." + ah.Name }

type ParseData struct {
	Errs   *perr.List
	Schema *schema.Parser

	File *pkginfo.File
	Func *ast.FuncDecl
	Dir  *directive.Directive
	Doc  string
}

// Parse parses the auth handler in the provided declaration.
// It may return nil on errors.
func Parse(d ParseData) *AuthHandler {
	decl, ok := d.Schema.ParseFuncDecl(d.File, d.Func)
	if !ok {
		return nil
	}

	ah := &AuthHandler{
		Decl: decl,
		Name: decl.Name,
		Doc:  d.Doc,
		Recv: decl.Recv,
	}

	sig := decl.Type
	numParams := len(sig.Params)

	// Validate the input
	if numParams != 2 {
		d.Errs.Add(errInvalidNumberParameters(numParams).AtGoNode(sig.AST.Params))

		if numParams < 2 {
			return ah
		}
	}

	numResults := len(sig.Results)
	if numResults < 2 || numResults > 3 {
		d.Errs.Add(errInvalidNumberResults(numResults).AtGoNode(sig.AST.Results))

		if numParams < 2 {
			return ah
		}
	}

	// First param should always be context.Context
	ctxParam := sig.Params[0]
	if !schemautil.IsNamed(ctxParam.Type, "context", "Context") {
		d.Errs.Add(errInvalidFirstParameter.AtGoNode(ctxParam.AST))
	}

	ah.Param = sig.Params[1].Type

	// Second param should be string, or a pointer to a named struct
	{
		param := ah.Param
		if schemautil.IsBuiltinKind(param, schema.String) {
			// All good
		} else if ref, ok := schemautil.ResolveNamedStruct(param, true); !ok {
			d.Errs.Add(ErrInvalidAuthSchemaType.AtGoNode(param.ASTExpr()))
		} else {
			validateStructFields(d.Errs, ref.Decl.Type.(schema.StructType))
		}
	}

	// First result must be auth.UID
	if uid := sig.Results[0]; !schemautil.IsBuiltinKind(uid.Type, schema.UserID) {
		d.Errs.Add(errInvalidFirstResult.AtGoNode(uid.AST.Type))
	}

	// If we have three results, the second one must be a pointer to a named struct.
	if numResults > 2 {
		if ref, ok := schemautil.ResolveNamedStruct(sig.Results[1].Type, true); !ok {
			d.Errs.Add(errInvalidSecondResult.AtGoNode(sig.Results[1].AST.Type))
		} else {
			ah.AuthData = option.Some(ref)
		}
	}

	return ah
}

// validateStructFields checks that the struct fields have the correct tags.
// and all fields are either a header or query string
func validateStructFields(errs *perr.List, ref schema.StructType) {
	fieldErr := ErrInvalidFieldTags

nextField:
	for _, field := range ref.Fields {
		// No tags, then we tell them they need to specify it
		if field.Tag.Len() == 0 {
			fieldErr = fieldErr.AtGoNode(field.AST, errors.AsError("you must specify a \"header\", \"query\", or \"cookie\" tag for this field"))
			continue
		}

		// Check each tag to see if we have a header or query tag
		for _, tagKey := range field.Tag.Keys() {
			errMsg := ""

			tag, err := field.Tag.Get(tagKey)
			if err == nil {
				switch tagKey {
				case "header", "query", "qs", "cookie":
					if tag.Name != "" && tag.Name != "-" {
						continue nextField
					} else {
						errMsg = "you must specify a name for this field"
					}
				}
			}

			fieldErr = fieldErr.AtGoNode(field.AST.Tag, errors.AsError(errMsg))
		}
	}

	if len(fieldErr.Locations) > 0 {
		errs.Add(fieldErr)
	}
}
