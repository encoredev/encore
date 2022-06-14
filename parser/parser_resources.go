package parser

import (
	"go/ast"
	"go/token"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
)

type pkgPath = string
type resourceCreator struct {
	funcName    string
	numTypeArgs int
}
type resourceParser struct {
	ResourceName string // The human read-able name of the resource
	CreationFunc string //
	DocsPage     string // The URL of the documentation page for the resource type
	Parse        func(*parser, *est.File, string, *ast.ValueSpec, *ast.CallExpr)
}

// resourceRegistry is a map of pkg path => creation function name
var resourceRegistry = map[pkgPath]map[resourceCreator]*resourceParser{}

// parseResources parses infrastructure resources declared in the packages.
func (p *parser) parseResources() {
	p.parseOldResources()

	for _, pkg := range p.pkgs {
		for _, file := range pkg.Files {
			ast.Walk(&resourceCreationVisitor{p, file, p.names[pkg].Files[file]}, file.AST)
		}
	}
}

type resourceCreationVisitor struct {
	p     *parser
	file  *est.File
	names *names.File
}

// Visit will walk the AST of a file looking for package level variable declarations made to resource creation functions
// as defined in the resourceRegistry.
//
// It hands off to VisitAndReportInvalidCreationCalls to walk any function bodies
func (f *resourceCreationVisitor) Visit(node ast.Node) (w ast.Visitor) {
	switch node := node.(type) {
	case *ast.GenDecl:
		if node.Tok == token.VAR {
			for _, spec := range node.Specs {
				walkSpec := true

				switch spec := spec.(type) {
				case *ast.ValueSpec:
					if len(spec.Names) == 1 && len(spec.Values) == 1 {
						if callExpr, ok := spec.Values[0].(*ast.CallExpr); ok {
							// Find if there's a resource type for this, and call it's parse function
							if parser := f.parserFor(callExpr.Fun); parser != nil {
								walkSpec = false

								parser.Parse(f.p, f.file, docNodeToString(node.Doc), spec, callExpr)
							}
						}
					}
				}

				if walkSpec {
					ast.Walk(walkerFunc(f.VisitAndReportInvalidCreationCalls), spec)
				}
			}

			// We don't want to visit the GenDecl node as we've already manually walked it, so we return nil here.
			return nil
		}

		return walkerFunc(f.VisitAndReportInvalidCreationCalls)
	case *ast.FuncDecl:
		return walkerFunc(f.VisitAndReportInvalidCreationCalls)
	default:
		return f
	}
}

// VisitAndReportInvalidCreationCalls walks the AST looking for calls to resource creation function, however
// if we encounter them, then we need to report an error as the resource creation function is only allowed to be used
// as a top-level variable declaration.
func (f *resourceCreationVisitor) VisitAndReportInvalidCreationCalls(node ast.Node) bool {
	switch node := node.(type) {
	case *ast.CallExpr:
		if parser := f.parserFor(node.Fun); parser != nil {
			f.p.errf(node.Pos(), "A %s must be declared as a package level variable. For more information please see %s\n", parser.ResourceName, parser.DocsPage)
			return false
		}

	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		if parser := f.parserFor(node); parser != nil {
			f.p.errf(node.Pos(), "%s can only be used to declare package level variables. For more information please see %s\n", parser.CreationFunc, parser.DocsPage)
			return false
		}
	}

	return true
}

func (f *resourceCreationVisitor) parserFor(node ast.Node) *resourceParser {
	numTypeArguments := 0

	// foo.bar[baz] is an index expression - so we want to unwrap the index expression
	// and foo.bar[baz, qux] is an index list expression
	switch idx := node.(type) {
	case *ast.IndexExpr:
		node = idx.X
		numTypeArguments = 1
	case *ast.IndexListExpr:
		node = idx.X
		numTypeArguments = len(idx.Indices)
	}

	pkgPath, objName := pkgObj(f.names, node)
	if pkgPath != "" && objName != "" {
		if packageResources, found := resourceRegistry[pkgPath]; found {
			if parser, found := packageResources[resourceCreator{objName, numTypeArguments}]; found {
				return parser
			}
		}
	}

	return nil
}
