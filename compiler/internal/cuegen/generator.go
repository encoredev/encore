package cuegen

import (
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/ast/astutil"
	"cuelang.org/go/cue/format"

	"encr.dev/parser"
	"encr.dev/parser/est"
)

// base off: https://github.com/cue-lang/cue/blob/06484a39d8d44c656212ac2d3b5589f8aa9db033/encoding/jsonschema/decode.go

type Generator struct {
	res *parser.Result
}

func NewGenerator(res *parser.Result) *Generator {
	return &Generator{
		res: res,
	}
}

// UserFacing generates a CUE file for the given service based.
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

	// Add all the topc level fields required by this service
	for _, configLoad := range svc.ConfigLoads {
		if err := service.registerTopLevelField(configLoad.ConfigStruct.Type); err != nil {
			return nil, err
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
