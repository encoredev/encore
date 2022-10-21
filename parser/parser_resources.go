package parser

import (
	"go/ast"
	"strings"

	"encr.dev/parser/dnsname"
	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
	"encr.dev/parser/internal/walker"
	"encr.dev/pkg/errinsrc/srcerrors"
)

// parseResources parses infrastructure resources declared in the packages.
// These are defined by calls to registerResource and registerResourceCreationParser.
func (p *parser) parseResources() {
	maxPhases := 0
	for _, res := range resourceTypes {
		if res.PhaseNum > maxPhases {
			maxPhases = res.PhaseNum
		}
	}

	for phase := 0; phase <= maxPhases; phase++ {
		for _, pkg := range p.pkgs {
			for _, file := range pkg.Files {
				walker.Walk(file.AST, &resourceCreationVisitor{p, file, p.names, phase})
			}
		}

		if p.errors.Len() > 0 {
			// Stop parsing phases if we encountere errors, as future phases will probably have errors which end up being
			// caused by these errors (such as pubusb.Subscriptions needing pubsub.Topics to be parsed first)
			break
		}
	}
}

type resourceCreationVisitor struct {
	p        *parser
	file     *est.File
	names    names.Application
	phaseNum int
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

						pkgResourceMap, found := f.p.resourceMap[f.file.Pkg.ImportPath]
						if !found {
							pkgResourceMap = make(map[string]est.Resource)
							f.p.resourceMap[f.file.Pkg.ImportPath] = pkgResourceMap
						}
						pkgResourceMap[ident.Name] = resource
					}

					f.file.Pkg.Resources = append(f.file.Pkg.Resources, resource)
				}
			} else {
				f.p.errf(
					node.Pos(),
					"A %s cannot be declared here, %s. For more information see %s",
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
			f.p.errf(node.Pos(), "%s.%s can only be called as a function to create a new instance and not referenced otherwise. For more information see %s", parser.Resource.PkgName, parser.Name, parser.Resource.Docs)
			return nil
		}
	}

	return f
}

func (f *resourceCreationVisitor) parserFor(node ast.Node) *resourceCreatorParser {
	pkgPath, objName, typeArgs := f.names.PackageLevelRef(f.file, node)
	if pkgPath != "" && objName != "" {
		if packageResources, found := resourceCreationRegistry[pkgPath]; found {
			if parser, found := packageResources[funcIdent{objName, len(typeArgs)}]; found {
				if parser.Resource.PhaseNum == f.phaseNum {
					return parser
				}
			}
		}
	}

	return nil
}

// resourceFor returns the resource that the given node references, or nil if it does not reference a resource
func (p *parser) resourceFor(file *est.File, node ast.Expr) est.Resource {
	pkgPath, objName, _ := p.names.PackageLevelRef(file, node)
	if pkgPath == "" {
		return nil
	}

	if idents, found := p.resourceMap[pkgPath]; found {
		if resource, found := idents[objName]; found {
			return resource
		}
	}

	return nil
}

// parseResourceName checks the given node is a string literal, and then checks it conforms
// to the DNS-1035 label spec:
//   - lowercase alpha-numeric, dashes
//   - must start and with a letter
//   - must not end with a dash
//   - must be between 1 and 63 characters long
//
// If an error is encountered, it will report a parse error and return an empty string
// otherwise it will return the parsed resource name
func (p *parser) parseResourceName(resourceType string, paramName string, node ast.Expr) string {
	name, ok := litString(node)
	if !ok {
		p.errInSrc(srcerrors.ResourceNameNotStringLiteral(p.fset, node, resourceType, paramName))
		return ""
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > dnsname.DNS1035LabelMaxLength {
		p.errInSrc(srcerrors.ResourceNameWrongLength(p.fset, node, resourceType, paramName, name))
		return ""
	}

	if !dnsname.Dns1035LabelRegexp.MatchString(name) {
		p.errInSrc(srcerrors.ResourceNameNotKebabCase(p.fset, node, resourceType, paramName, name))
		return ""
	}

	return name
}
