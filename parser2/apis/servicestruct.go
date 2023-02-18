package apis

import (
	"go/ast"
	"go/token"

	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
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

// parseServiceStruct parses the pkg for any declared encore:service structs.
func (p *Parser) parseServiceStruct(file *pkginfo.File, gd *ast.GenDecl, dir *serviceDirective, doc string) *ServiceStruct {
	// We only support encore:service directives directly on the type declaration,
	// not on a group of type declarations.
	if len(gd.Specs) != 1 {
		p.c.Errs.Add(dir.Pos(), "invalid encore directive location (expected on declaration, not group)")
		if len(gd.Specs) == 0 {
			// We can't continue without at least one spec.
			p.c.Errs.Bailout()
		}
	}

	spec := gd.Specs[0].(*ast.TypeSpec)
	declInfo := file.Pkg.Names().PkgDecls[spec.Name.Name]
	decl := p.schema.ParseTypeDecl(declInfo)

	ss := &ServiceStruct{
		Decl: decl,
		Doc:  doc,
	}

	// Find the init function for this service struct, if any.
	initFunc := file.Pkg.Names().PkgDecls["init"+ss.Decl.Name]
	if initFunc != nil && initFunc.Type == token.FUNC {
		ss.Init = option.Some(p.schema.ParseFuncDecl(initFunc.File, initFunc.Func))
	}

	p.validateServiceStruct(ss)
	return ss
}

// validateServiceStruct validates that the service struct and associated init function
// has the correct structure.
func (p *Parser) validateServiceStruct(ss *ServiceStruct) {
	if len(ss.Decl.TypeParams) > 0 {
		p.c.Errs.Add(ss.Decl.AST.Pos(), "encore:service types cannot be defined as generic types")
	}

	if ss.Init.IsPresent() {
		initFunc := ss.Init.MustGet()
		if len(initFunc.TypeParams) > 0 {
			p.c.Errs.Add(initFunc.AST.Pos(), "service init function cannot be defined as generic functions")
		}
		if len(initFunc.Type.Params) > 0 {
			p.c.Errs.Add(initFunc.AST.Pos(), "service init function cannot have parameters")
		}

		// Ensure the return type is (*T, error) where T is the service struct.
		if len(initFunc.Type.Results) != 2 {
			// Wrong number of returns
			p.c.Errs.Addf(initFunc.AST.Pos(), "service init function must return (*%s, error)", ss.Decl.Name)
		} else if result, n := schema.Deref(initFunc.Type.Results[0].Type); n != 1 || !schema.IsNamed(result, ss.Decl.Pkg.ImportPath, ss.Decl.Name) {
			// First type is not *T
			p.c.Errs.Addf(initFunc.AST.Pos(), "service init function must return (*%s, error)", ss.Decl.Name)
		} else if !schema.IsBuiltinKind(initFunc.Type.Results[1].Type, schema.Error) {
			// Second type is not builtin error.
			p.c.Errs.Addf(initFunc.AST.Pos(), "service init function must return (*%s, error)", ss.Decl.Name)
		}
	}
}
