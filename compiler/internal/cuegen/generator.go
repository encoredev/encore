package cuegen

import (
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/ast/astutil"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/token"

	"encr.dev/parser"
	"encr.dev/parser/est"
)

type Generator struct {
	res *parser.Result
}

func NewGenerator(res *parser.Result) *Generator {
	return &Generator{
		res: res,
	}
}

// UserFacing generates a CUE file for the given service.
//
// It includes constraints and requirements based on the types passed to `encore.dev/config.Load[T]()`
// within the service.
func (g *Generator) UserFacing(svc *est.Service) ([]byte, error) {
	if len(svc.ConfigLoads) == 0 {
		return nil, nil
	}

	// Create a base file
	service := &service{
		g:             g,
		svc:           svc,
		file:          &ast.File{},
		neededImports: make(map[string]string),
		fieldLookup:   make(map[string]*ast.Field),
		typeUsage:     newDefinitionGenerator(g.res.Meta.Decls),
	}

	// Count the number of times each named type is used
	// this allows us to determine if we inline the named type
	// or create and use a Definition
	for _, configLoad := range svc.ConfigLoads {
		if err := service.countNamedUsagesAndCollectImports(configLoad.ConfigStruct.Type); err != nil {
			return nil, err
		}
	}

	// Add all the top level fields required by this service
	for _, configLoad := range svc.ConfigLoads {
		if err := service.registerTopLevelField(configLoad.ConfigStruct.Type); err != nil {
			return nil, err
		}
	}

	// For the first top level field in a service, if it's not go a comment above it, then we want to put it's label position
	// as a new section. This forces a blank line between the type decelerations and the first field.
	if len(service.topLevelFields) > 0 {
		if field, ok := service.topLevelFields[0].(*ast.Field); ok {
			if !hasCommentInPosition(field, 0) {
				if ident, ok := field.Label.(*ast.Ident); ok {
					ident.NamePos = token.NewSection.Pos()
				}
			}
		}
	}

	// Now generate the CUE
	if err := service.generateCue(); err != nil {
		return nil, err
	}

	// Cleanup the generated AST
	if err := astutil.Sanitize(service.file); err != nil {
		return nil, err
	}

	// Format the AST into a set of bytes we can write
	return format.Node(
		service.file,
		format.Simplify(),
		format.UseSpaces(4),
	)
}
