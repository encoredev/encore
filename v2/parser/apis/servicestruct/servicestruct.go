package servicestruct

import (
	"fmt"
	"go/ast"
	"go/token"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/option"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/apis/internal/directive"
	"encr.dev/v2/parser/internal/utils"
	"encr.dev/v2/parser/resource"
)

// ServiceStruct describes a dependency injection struct for a service.
type ServiceStruct struct {
	Decl *schema.TypeDecl // decl is the type declaration
	Doc  string

	// Init is the function for initializing this group.
	// It is nil if there is no initialization function.
	Init option.Option[*schema.FuncDecl]
}

func (ss *ServiceStruct) Kind() resource.Kind       { return resource.ServiceStruct }
func (ss *ServiceStruct) Package() *pkginfo.Package { return ss.Decl.File.Pkg }
func (ss *ServiceStruct) Pos() token.Pos            { return ss.Decl.AST.Pos() }
func (ss *ServiceStruct) End() token.Pos            { return ss.Decl.AST.End() }
func (ss *ServiceStruct) SortKey() string {
	return ss.Decl.File.Pkg.ImportPath.String() + "." + ss.Decl.Name
}

type ParseData struct {
	Errs   *perr.List
	Schema *schema.Parser

	File *pkginfo.File
	Decl *ast.GenDecl
	Dir  *directive.Directive
	Doc  string
}

// Parse parses the service struct in the provided type declaration.
func Parse(d ParseData) *ServiceStruct {
	// We don't allow anything on the directive besides "encore:service".
	directive.Validate(d.Errs, d.Dir, directive.ValidateSpec{})

	// We only support encore:service directives directly on the type declaration,
	// not on a group of type declarations.
	if len(d.Decl.Specs) != 1 {
		d.Errs.Add(errInvalidDirectivePlacement.AtGoNode(d.Dir.AST))
		if len(d.Decl.Specs) == 0 {
			// We can't continue without at least one spec.
			d.Errs.Bailout()
		}
	}

	spec := d.Decl.Specs[0].(*ast.TypeSpec)
	declInfo := d.File.Pkg.Names().PkgDecls[spec.Name.Name]
	decl := d.Schema.ParseTypeDecl(declInfo)

	ss := &ServiceStruct{
		Decl: decl,
		Doc:  d.Doc,
	}

	// Find the init function for this service struct, if any.
	initFunc := d.File.Pkg.Names().PkgDecls["init"+ss.Decl.Name]
	if initFunc != nil && initFunc.Type == token.FUNC {
		init, ok := d.Schema.ParseFuncDecl(initFunc.File, initFunc.Func)
		if ok {
			ss.Init = option.Some(init)
		}
	}

	validateServiceStruct(d, ss)
	return ss
}

// validateServiceStruct validates that the service struct and associated init function
// has the correct structure.
func validateServiceStruct(d ParseData, ss *ServiceStruct) {
	if len(ss.Decl.TypeParams) > 0 {
		d.Errs.Add(errServiceStructMustNotBeGeneric.AtGoNode(ss.Decl.TypeParams[0].AST))
	}

	ss.Init.ForAll(func(initFunc *schema.FuncDecl) {
		if len(initFunc.TypeParams) > 0 {
			d.Errs.Add(errServiceInitCannotBeGeneric.AtGoNode(initFunc.TypeParams[0].AST))
		}
		if len(initFunc.Type.Params) > 0 {
			d.Errs.Add(errServiceInitCannotHaveParams.AtGoNode(initFunc.Type.Params[0].AST))
		}

		// Ensure the return type is (*T, error) where T is the service struct.
		if len(initFunc.Type.Results) != 2 {
			// Wrong number of returns
			d.Errs.Add(errServiceInitInvalidReturnType(ss.Decl.Name).AtGoNode(initFunc.AST))
		} else if result, n := schemautil.Deref(initFunc.Type.Results[0].Type); n != 1 || !schemautil.IsNamed(result, ss.Decl.File.Pkg.ImportPath, ss.Decl.Name) {
			// First type is not *T
			d.Errs.Add(
				errServiceInitInvalidReturnType(ss.Decl.Name).
					AtGoNode(initFunc.AST, errors.AsError(
						fmt.Sprintf("got %s", utils.PrettyPrint(initFunc.Type.Results[0].Type.ASTExpr())),
					)),
			)
		} else if !schemautil.IsBuiltinKind(initFunc.Type.Results[1].Type, schema.Error) {
			// Second type is not builtin error.
			d.Errs.Add(
				errServiceInitInvalidReturnType(ss.Decl.Name).
					AtGoNode(initFunc.AST, errors.AsError(
						fmt.Sprintf("got %s", utils.PrettyPrint(initFunc.Type.Results[1].Type.ASTExpr())),
					)),
			)
		}
	})
}
