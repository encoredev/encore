package parser

import (
	"go/ast"
	"go/token"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/names"
)

type pkgPath = string

type resource struct {
	Type    est.ResourceType
	Name    string
	Docs    string
	PkgName string
	PkgPath string
}

var resourceTypes = map[est.ResourceType]*resource{}

func registerResource(resourceType est.ResourceType, name string, docs string, pkgName string, pkgPath string) {
	defaultTrackedPackages[pkgPath] = pkgName
	resourceTypes[resourceType] = &resource{
		Type:    resourceType,
		Name:    name,
		Docs:    docs,
		PkgName: pkgName,
		PkgPath: pkgPath,
	}
}

type funcIdent struct {
	funcName    string
	numTypeArgs int
}
type resourceCreatorParser struct {
	Resource *resource
	Name     string // The name of the function this is registered against
	Parse    func(*parser, *est.File, string, *ast.ValueSpec, *ast.CallExpr)
}

// resourceCreationRegistry is a map of pkg path => creation function => parser
var resourceCreationRegistry = map[pkgPath]map[funcIdent]*resourceCreatorParser{}

func registerResourceCreationParser(resource est.ResourceType, funcName string, numTypeArgs int, parse func(*parser, *est.File, string, *ast.ValueSpec, *ast.CallExpr)) {
	res, ok := resourceTypes[resource]
	if !ok {
		panic("registerResourceCreationParser: unknown resource type")
	}

	if _, found := resourceCreationRegistry[res.PkgPath]; !found {
		resourceCreationRegistry[res.PkgPath] = map[funcIdent]*resourceCreatorParser{}
	}

	resourceCreationRegistry[res.PkgPath][funcIdent{funcName, numTypeArgs}] = &resourceCreatorParser{
		Resource: res,
		Name:     funcName,
		Parse:    parse,
	}
}

type resourceUsageParser struct {
	Resource         *resource
	Name             string
	AllowedLocations locations.Filters
	Parse            func(*parser, *est.File, est.Resource, *ast.CallExpr)
}

// resourceUsageRegistry is a map of resource type => function on that resource => parser
var resourceUsageRegistry = map[est.ResourceType]map[string]*resourceUsageParser{}

func registerResourceUsageParser(resourceType est.ResourceType, name string, parse func(*parser, *est.File, est.Resource, *ast.CallExpr), allowedLocations ...locations.Filter) {
	res, ok := resourceTypes[resourceType]
	if !ok {
		panic("registerResourceCreationParser: unknown resource type")
	}

	if _, found := resourceUsageRegistry[resourceType]; !found {
		resourceUsageRegistry[resourceType] = map[string]*resourceUsageParser{}
	}

	resourceUsageRegistry[resourceType][name] = &resourceUsageParser{
		Resource:         res,
		Name:             name,
		AllowedLocations: allowedLocations,
		Parse:            parse,
	}
}

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
// as defined in the resourceCreationRegistry.
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
			f.p.errf(node.Pos(), "A %s must be declared as a package level variable. For more information please see %s\n", parser.Resource.Name, parser.Resource.Docs)
			return false
		}

	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		if parser := f.parserFor(node); parser != nil {
			f.p.errf(node.Pos(), "%s.%s can only be used to declare package level variables. For more information please see %s\n", parser.Resource.PkgName, parser.Name, parser.Resource.Docs)
			return false
		}
	}

	return true
}

func (f *resourceCreationVisitor) parserFor(node ast.Node) *resourceCreatorParser {
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
		if packageResources, found := resourceCreationRegistry[pkgPath]; found {
			if parser, found := packageResources[funcIdent{objName, numTypeArguments}]; found {
				return parser
			}
		}
	}

	return nil
}
