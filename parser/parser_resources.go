package parser

import (
	"go/ast"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
	"encr.dev/parser/internal/walker"
)

// parseResources parses infrastructure resources declared in the packages.
// These are defined by calls to registerResource and registerResourceCreationParser.
func (p *parser) parseResources() {
	for _, pkg := range p.pkgs {
		for _, file := range pkg.Files {
			walker.Walk(file.AST, &resourceCreationVisitor{p, file, p.names})
		}
	}
}

type resourceCreationVisitor struct {
	p     *parser
	file  *est.File
	names names.Application
}

// Visit will walk the AST of a file looking for package level variable declarations made to resource creation functions
// as defined in the resourceCreationRegistry.
//
// It hands off to VisitAndReportInvalidCreationCalls to walk any function bodies
func (f *resourceCreationVisitor) Visit(cursor *walker.Cursor) (w walker.Visitor) {
	switch node := cursor.Node().(type) {
	case *ast.CallExpr:
		if parser := f.parserFor(node.Fun); parser != nil {
			if parser.AllowedLocations.Allowed(cursor.Location()) {
				// Identify the variable name from the value spec
				var ident *ast.Ident
				if spec, ok := cursor.Parent().(*ast.ValueSpec); ok {
					for i := 0; i < len(spec.Names); i++ {
						if spec.Values[i] == node {
							ident = spec.Names[i]
							break
						}
					}

					if ident == nil {
						f.p.errf(
							spec.Pos(),
							"Unable to find the identifier that the %s is bound to.",
							parser.Resource.Name,
						)
						return nil
					}
				}

				// If the parser allows resource to be created here, let's call parse it
				// and then record the resource that was created
				if resource := parser.Parse(f.p, f.file, cursor, ident, node); resource != nil {

					if ident != nil {
						f.file.References[ident] = &est.Node{
							Type: resource.NodeType(),
							Res:  resource,
						}
					}

					f.file.Pkg.Resources = append(f.file.Pkg.Resources, resource)
				}
			} else {
				f.p.errf(
					node.Pos(),
					"A %s can not be declared here, %s. For more information please see %s",
					parser.Resource.Name,
					parser.AllowedLocations.Describe("they", "declared"),
					parser.Resource.Docs,
				)
			}

			return nil
		}

	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		// If we find a selector (`a.foo`) an index (`a.foo[bar]`) or an index list (`a.foo[bar, baz]`)
		// then we want to check if that references a resource creation function and if so
		// report an error, as all valid usages should already have been parsed and returned
		if parser := f.parserFor(node); parser != nil {
			f.p.errf(node.Pos(), "%s.%s can only be called as a function to create a new instance and not referenced otherwise. For more information please see %s", parser.Resource.PkgName, parser.Name, parser.Resource.Docs)
			return nil
		}
	}

	return f
}

func (f *resourceCreationVisitor) parserFor(node ast.Node) *resourceCreatorParser {
	pkgPath, objName, typeArgs := f.names.PkgObjRef(f.file, node)
	if pkgPath != "" && objName != "" {
		if packageResources, found := resourceCreationRegistry[pkgPath]; found {
			if parser, found := packageResources[funcIdent{objName, len(typeArgs)}]; found {
				return parser
			}
		}
	}

	return nil
}
