package cuegen

import (
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/ast/astutil"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/token"

	"encr.dev/v2/app"
	"encr.dev/v2/parser/infra/config"
)

type Generator struct {
	desc *app.Desc
}

func NewGenerator(desc *app.Desc) *Generator {
	return &Generator{
		desc: desc,
	}
}

// UserFacing generates a CUE file for the given service.
//
// It includes constraints and requirements based on the types passed to `encore.dev/config.Load[T]()`
// within the service.
func (g *Generator) UserFacing(svc *app.Service) ([]byte, error) {
	var loads []*config.Load
	for res := range svc.ResourceUsage {
		if cfg, ok := res.(*config.Load); ok {
			loads = append(loads, cfg)
		}
	}
	if len(loads) == 0 {
		return nil, nil
	}

	// Create a base file
	service := &serviceFile{
		g:             g,
		svc:           svc,
		file:          &ast.File{},
		neededImports: make(map[string]string),
		fieldLookup:   make(map[string]*ast.Field),
		typeUsage:     newDefinitionGenerator(),
	}

	// Count the number of times each named type is used
	// this allows us to determine if we inline the named type
	// or create and use a Definition
	for _, configLoad := range loads {
		service.countNamedUsagesAndCollectImports(configLoad.Type)
	}

	// Add all the top level fields required by this service
	for _, configLoad := range loads {
		service.registerTopLevelField(configLoad.Type)
	}

	// For the first top level field in a service, if it's not go a comment above it, then we want to put it's label position
	// as a new section. This forces a blank line between the type declarations and the first field.
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
	service.generateCue()

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
