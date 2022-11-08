package parser

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
	"encr.dev/parser/internal/walker"
	"encr.dev/pkg/errinsrc/srcerrors"
)

// parseResources parses infrastructure resources declared in the packages.
// These are defined by calls to registerResource and registerResourceCreationParser.
func (p *parser) parseResources() {
	maxPhases := 0
	pkgsByPhase := make(map[int][]string)

	for _, res := range resourceTypes {
		if res.PhaseNum > maxPhases {
			maxPhases = res.PhaseNum
		}
		if !slices.Contains(pkgsByPhase[res.PhaseNum], res.PkgPath) {
			pkgsByPhase[res.PhaseNum] = append(pkgsByPhase[res.PhaseNum], res.PkgPath)
		}
	}

	for phase := 0; phase <= maxPhases; phase++ {
		interestingPkgs := pkgsByPhase[phase]
		for _, pkg := range p.pkgs {
			// If the package does not contain any imports we care about in this phase, skip it.
			if !mapContainsAny(pkg.Imports, interestingPkgs) {
				continue
			}

			for _, file := range pkg.Files {
				if !mapContainsAny(file.Imports, interestingPkgs) {
					continue
				}
				walker.Walk(file.AST, &resourceCreationVisitor{p, file, p.names, phase})
			}
		}

		if p.errors.Len() > 0 {
			// Stop parsing phases if we encounter errors, as future phases will probably have errors which end up being
			// caused by these errors (such as pubsub.Subscriptions needing pubsub.Topics to be parsed first)
			break
		}
	}
}

func mapContainsAny(imports map[string]bool, pkgsPaths []string) bool {
	for _, pkg := range pkgsPaths {
		if imports[pkg] {
			return true
		}
	}
	return false
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

const resourceNameMaxLength int = 63

type resourceNameSpec struct {
	regexp         *regexp.Regexp
	invalidNameErr func(fset *token.FileSet, node ast.Node, resourceType, paramName, name string) error
	reservedErr    func(fset *token.FileSet, node ast.Node, resourceType, paramName, name, reservedPrefix string) error
}

var kebabName = resourceNameSpec{
	regexp:         regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`),
	invalidNameErr: srcerrors.ResourceNameNotKebabCase,
	reservedErr: func(fset *token.FileSet, node ast.Node, resourceType, paramName, name, reservedPrefix string) error {
		return srcerrors.ResourceNameReserved(fset, node, resourceType, paramName, name, reservedPrefix, false)
	},
}

var snakeName = resourceNameSpec{
	regexp:         regexp.MustCompile(`^[a-z]([_a-z0-9]*[a-z0-9])?$`),
	invalidNameErr: srcerrors.ResourceNameNotSnakeCase,
	reservedErr: func(fset *token.FileSet, node ast.Node, resourceType, paramName, name, reservedPrefix string) error {
		return srcerrors.ResourceNameReserved(fset, node, resourceType, paramName, name, reservedPrefix, true)
	},
}

// parseResourceName checks the given node is a string literal, and then checks it conforms
// to the given spec.
//
// If an error is encountered, it will report a parse error and return an empty string
// otherwise it will return the parsed resource name
func (p *parser) parseResourceName(resourceType string, paramName string, node ast.Expr, nameSpec resourceNameSpec, reservedPrefix string) string {
	name, ok := litString(node)
	if !ok {
		p.errInSrc(srcerrors.ResourceNameNotStringLiteral(p.fset, node, resourceType, paramName))
		return ""
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > resourceNameMaxLength {
		p.errInSrc(srcerrors.ResourceNameWrongLength(p.fset, node, resourceType, paramName, name))
		return ""
	}

	if !nameSpec.regexp.MatchString(name) {
		p.errInSrc(nameSpec.invalidNameErr(p.fset, node, resourceType, paramName, name))
		return ""
	} else if reservedPrefix != "" && strings.HasPrefix(name, reservedPrefix) {
		p.errInSrc(nameSpec.reservedErr(p.fset, node, resourceType, paramName, name, reservedPrefix))
		return ""
	}

	return name
}
