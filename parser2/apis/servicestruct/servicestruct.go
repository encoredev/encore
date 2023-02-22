package servicestruct

import (
	"go/ast"
	"go/token"

	"encr.dev/parser2/apis/directive"
	"encr.dev/parser2/internal/perr"
	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
	"encr.dev/parser2/internal/schema/schemautil"
	"encr.dev/pkg/option"
)

// ServiceStruct describes a dependency injection struct for a service.
type ServiceStruct struct {
	Decl *schema.TypeDecl // decl is the type declaration
	Doc  string

	// Init is the function for initializing this group.
	// It is nil if there is no initialization function.
	Init option.Option[*schema.FuncDecl]
}

type ParseData struct {
	Errs   *perr.List
	Schema *schema.Parser

	File *pkginfo.File
	Decl *ast.GenDecl
	Dir  directive.Directive
	Doc  string
}

// Parse parses the service struct in the provided type declaration.
func Parse(d ParseData) *ServiceStruct {
	// We don't allow anything on the directive besides "encore:service".
	if err := directive.Validate(d.Dir, directive.ValidateSpec{}); err != nil {
		d.Errs.Addf(d.Dir.AST.Pos(), "invalid encore:service directive: %v", err)
	}

	// We only support encore:service directives directly on the type declaration,
	// not on a group of type declarations.
	if len(d.Decl.Specs) != 1 {
		d.Errs.Add(d.Dir.AST.Pos(), "invalid encore:service directive location (expected on declaration, not group)")
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
		ss.Init = option.Some(d.Schema.ParseFuncDecl(initFunc.File, initFunc.Func))
	}

	validateServiceStruct(d, ss)
	return ss
}

// validateServiceStruct validates that the service struct and associated init function
// has the correct structure.
func validateServiceStruct(d ParseData, ss *ServiceStruct) {
	if len(ss.Decl.TypeParams) > 0 {
		d.Errs.Add(ss.Decl.AST.Pos(), "encore:service types cannot be defined as generic types")
	}

	if ss.Init.IsPresent() {
		initFunc := ss.Init.MustGet()
		if len(initFunc.TypeParams) > 0 {
			d.Errs.Add(initFunc.AST.Pos(), "service init function cannot be defined as generic functions")
		}
		if len(initFunc.Type.Params) > 0 {
			d.Errs.Add(initFunc.AST.Pos(), "service init function cannot have parameters")
		}

		// Ensure the return type is (*T, error) where T is the service struct.
		if len(initFunc.Type.Results) != 2 {
			// Wrong number of returns
			d.Errs.Addf(initFunc.AST.Pos(), "service init function must return (*%s, error)", ss.Decl.Name)
		} else if result, n := schemautil.Deref(initFunc.Type.Results[0].Type); n != 1 || !schemautil.IsNamed(result, ss.Decl.File.Pkg.ImportPath, ss.Decl.Name) {
			// First type is not *T
			d.Errs.Addf(initFunc.AST.Pos(), "service init function must return (*%s, error)", ss.Decl.Name)
		} else if !schemautil.IsBuiltinKind(initFunc.Type.Results[1].Type, schema.Error) {
			// Second type is not builtin error.
			d.Errs.Addf(initFunc.AST.Pos(), "service init function must return (*%s, error)", ss.Decl.Name)
		}
	}
}
