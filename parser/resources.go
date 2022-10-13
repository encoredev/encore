package parser

import (
	"go/ast"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/walker"
)

// resource represents a type of resource that can be created & used within an Encore application,
// such as a CronJob, PubSub Topic or database instance.
type resource struct {
	Type     est.ResourceType // The type of resource in the Encore Syntax Tree (EST)
	Name     string           // The human-readable name for the type of resource
	Docs     string           // The link to the docs page for this resource on the Encore website
	PkgName  string           // The name of the Go package that the resource API is defined in
	PkgPath  string           // The path to the Go package that the resource API is defined in
	PhaseNum int              // What phase of resource parsing should we be in when we look for this resource type
}

// resourceTypes maps resource types to their corresponding resource structs.
var resourceTypes = map[est.ResourceType]*resource{}

// registerResource is used by resources to add themselves to the map above, but also to ensure that
// we track the usage of the package the resource is declared in
func registerResource(resourceType est.ResourceType, name string, docs string, pkgName string, pkgPath string, dependsOn ...est.ResourceType) {
	res := &resource{
		Type:     resourceType,
		Name:     name,
		Docs:     docs,
		PkgName:  pkgName,
		PkgPath:  pkgPath,
		PhaseNum: 0,
	}
	for _, dependancy := range dependsOn {
		if resourceTypes[dependancy].PhaseNum >= res.PhaseNum {
			res.PhaseNum = resourceTypes[dependancy].PhaseNum + 1
		}
	}

	defaultTrackedPackages[pkgPath] = pkgName
	resourceTypes[resourceType] = res
}

// creatorParserFunc is a function that parses a call expression to create a resource.
//
// The *ast.Ident may be nil if the call expression was not bound to a variable.
//
// It should return nil if the resource creation failed, or the resource if it succeeded.
type creatorParserFunc func(*parser, *est.File, *walker.Cursor, *ast.Ident, *ast.CallExpr) est.Resource

// resourceCreatorParser is a struct that contains the information needed to parse a call expression make to create a
// new resource
type resourceCreatorParser struct {
	Resource         *resource         // The type of resource this function creates
	Name             string            // The name of the function this is registered against
	AllowedLocations locations.Filters // The locations this function is allowed to be called from
	Parse            creatorParserFunc // The function to call to parse the call expression
}

// funcIdent is a helper struct that represents a function identifier (name and number of type arguments)
type funcIdent struct {
	funcName    string
	numTypeArgs int
}

// resourceCreationRegistry is a map of pkg path => function name => parser struct of resource creation parsers
var resourceCreationRegistry = map[string]map[funcIdent]*resourceCreatorParser{}

// registerResourceCreationParser is used by resources to register any function calls which will create them.
//
// It will panic if the resource type is not registered already.
func registerResourceCreationParser(resource est.ResourceType, funcName string, numTypeArgs int, parse creatorParserFunc, allowedLocations ...locations.Filter) {
	res, ok := resourceTypes[resource]
	if !ok {
		panic("registerResourceCreationParser: unknown resource type")
	}

	if _, found := resourceCreationRegistry[res.PkgPath]; !found {
		resourceCreationRegistry[res.PkgPath] = map[funcIdent]*resourceCreatorParser{}
	}

	resourceCreationRegistry[res.PkgPath][funcIdent{funcName, numTypeArgs}] = &resourceCreatorParser{
		Resource:         res,
		Name:             funcName,
		AllowedLocations: allowedLocations,
		Parse:            parse,
	}
}

// resourceUsageParserFunc is a function that parses a call expression to a receiver of a resource instance.
type resourceUsageParserFunc func(*parser, *est.File, est.Resource, *walker.Cursor, *ast.CallExpr)

// resourceUsageParser is a struct that contains the information needed to parse a call expression to a receiver of a
// resource instance
type resourceUsageParser struct {
	Resource         *resource               // The type of resource this function uses
	Name             string                  // The name of the receiver function for this resource type
	AllowedLocations locations.Filters       // The locations that this function is allowed to be called from
	Parse            resourceUsageParserFunc // The function to call to parse the call expression
}

// resourceUsageRegistry is a map of resource type => function on that resource => parser
var resourceUsageRegistry = map[est.ResourceType]map[string]*resourceUsageParser{}

// registerResourceUsageParser is used by resources to register any function calls which will use a resource instance.
//
// It will panic if the resource type is not registered already.
func registerResourceUsageParser(resourceType est.ResourceType, name string, parse resourceUsageParserFunc, allowedLocations ...locations.Filter) {
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

type resourceReferenceParserFunc func(*parser, *est.File, est.Resource, *walker.Cursor)

type resourceReferenceParser struct {
	Resource *resource                   // The type of resource being referenced
	Parse    resourceReferenceParserFunc // The function to call to parse the reference
}

// resourceReferenceRegistry is a map of resource type => parser
var resourceReferenceRegistry = map[est.ResourceType]*resourceReferenceParser{}

// registerResourceReferenceParser is used by resources to register any references to a resource instance.
//
// It will panic if the resource type is not registered already.
func registerResourceReferenceParser(resourceType est.ResourceType, parse resourceReferenceParserFunc) {
	res, ok := resourceTypes[resourceType]
	if !ok {
		panic("registerResourceReferenceParser: unknown resource type")
	}

	resourceReferenceRegistry[resourceType] = &resourceReferenceParser{
		Resource: res,
		Parse:    parse,
	}
}
