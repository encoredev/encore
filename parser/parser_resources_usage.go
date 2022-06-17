package parser

import (
	"fmt"
	"go/ast"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
	"encr.dev/parser/internal/walker"
)

func (p *parser) parseResourceUsage() {
	// For all resources defined, store them in a map per package for faster lookup
	resourceMap := make(map[string]map[string]est.Resource, len(p.pkgs)) // path -> name -> resource
	for _, pkg := range p.pkgs {
		resources := make(map[string]est.Resource, len(pkg.Resources))
		resourceMap[pkg.ImportPath] = resources
		for _, res := range pkg.Resources {
			id := res.Ident()
			resources[id.Name] = res
		}
	}

	for _, pkg := range p.pkgs {
		for _, file := range pkg.Files {
			walker.Walk(file.AST, &resourceUsageVisitor{p, file, p.names, resourceMap})
		}
	}
}

type resourceUsageVisitor struct {
	p         *parser
	file      *est.File
	names     names.Application
	resources map[string]map[string]est.Resource
}

func (r *resourceUsageVisitor) Visit(cursor *walker.Cursor) (w walker.Visitor) {
	node := cursor.Node()

	switch node := node.(type) {
	case *ast.CallExpr:
		resource, parser := r.resourceAndFuncFor(node)
		if parser != nil {
			if parser.AllowedLocations.Allowed(cursor.Location()) {
				parser.Parse(r.p, r.file, resource, cursor, node)
			} else {
				call := fmt.Sprintf("`%s.%s`", resource.Ident().Name, parser.Name)
				r.p.errf(node.Pos(),
					"You cannot call %s here, %s️. For more information see %s",
					call,
					parser.AllowedLocations.Describe("it", "called"),
					parser.Resource.Docs,
				)
			}

			return nil
		}

	case *ast.SelectorExpr:
		if resource := r.resourceFor(node); resource != nil && resource.AllowOnlyParsedUsage() {
			// If the resource type isn't registered, for now this is Ok as we have SQLDB resources that are not tracked
			if res, found := resourceTypes[resource.Type()]; found {
				r.p.errf(node.Pos(),
					"A %s cannot be referenced, apart from when calling a method on it. For more information see %s",
					res.Name,
					res.Docs,
				)
			}
			return nil
		}
	}

	return r
}

func (r *resourceUsageVisitor) resourceAndFuncFor(callExpr *ast.CallExpr) (est.Resource, *resourceUsageParser) {
	if sel, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		var arg ast.Node

		// We expect either a double selector; (`[[pkg.name].func]`) and we want to identify the resource from the
		// inner selector (`[pkg.name]`), then identify the function from the outer selector (`func`).
		//
		// Or for a resource defined in the same package, we expect a single selector; (`name.func`) and we
		// we know the pkg is the same as the file, so we can identify the resource from the name and the function
		if nested, ok := sel.X.(*ast.SelectorExpr); ok {
			arg = nested
		} else if ident, ok := sel.X.(*ast.Ident); ok {
			arg = ident
		} else {
			return nil, nil
		}

		if resource := r.resourceFor(arg); resource != nil {
			if resourceFuncs, found := resourceUsageRegistry[resource.Type()]; found {
				if parser, found := resourceFuncs[sel.Sel.Name]; found {
					return resource, parser
				}
			}
		}
	}

	return nil, nil
}

func (r *resourceUsageVisitor) resourceFor(node ast.Node) est.Resource {
	pkgPath, objName, _ := r.names.PkgObjRef(r.file, node)
	if pkgPath == "" {
		return nil
	}

	if idents, found := r.resources[pkgPath]; found {
		if resource, found := idents[objName]; found {
			return resource
		}
	}

	return nil
}
