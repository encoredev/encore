package parser

import (
	"go/ast"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/walker"
	"encr.dev/pkg/errinsrc/srcerrors"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

const configImportPath = "encore.dev/config"

func init() {
	registerResource(
		est.ConfigResource,
		"config",
		"https://encore.dev/docs/develop/config",
		"config",
		configImportPath,
	)

	registerResourceCreationParser(
		est.ConfigResource,
		"Load", 1,
		(*parser).parseConfigLoad,
		locations.AllowedIn(locations.Variable).ButNotIn(locations.Function),
	)

	registerResourceReferenceParser(
		est.ConfigResource,
		(*parser).parseConfigReference,
	)

	registerTypeResolver(configImportPath, (*parser).resolveConfigTypes)
}

func (p *parser) parseConfigLoad(file *est.File, _ *walker.Cursor, ident *ast.Ident, callExpr *ast.CallExpr) est.Resource {
	// Resolve the named struct used for the config type
	configType := p.resolveParameter("config type", file.Pkg, file, getTypeArguments(callExpr.Fun)[0], true)
	if configType == nil {
		return nil
	}

	// Validate the call to configLoad
	svc := file.Pkg.Service
	if svc == nil {
		p.errInSrc(srcerrors.ConfigOnlyLoadedFromService(p.fset, callExpr))
		return nil
	}
	if svc.Root != file.Pkg {
		p.errInSrc(srcerrors.ConfigMustBeTopLevelPackage(p.fset, callExpr))
		return nil
	}
	if len(callExpr.Args) > 0 {
		p.errInSrc(srcerrors.ConfigLoadNoArguments(p.fset, callExpr))
	}

	estNode := &est.Config{
		Svc:          file.Pkg.Service,
		DeclFile:     file,
		IdentAST:     ident,
		FuncCall:     callExpr,
		ConfigStruct: configType,
	}
	svc.ConfigLoads = append(svc.ConfigLoads, estNode)

	return estNode
}

func (p *parser) parseConfigReference(file *est.File, resource est.Resource, cursor *walker.Cursor) {
	config := resource.(*est.Config)
	if file.Pkg.Service != config.Svc {
		p.errInSrc(srcerrors.ConfigOnlyReferencedSameService(p.fset, cursor.Node(), config.IdentAST))
		return
	}
}

func (p *parser) resolveConfigTypes(selector *ast.SelectorExpr, ident *ast.Ident, typeParameters typeParameterLookup) *schema.Type {
	switch ident.Name {
	case "Value":
		return &schema.Type{
			Typ: &schema.Type_Config{
				Config: &schema.ConfigValue{
					IsValuesList: false,
					Elem:         nil, // we set this to nil here, and expect the type resolver to set it later
				},
			},
		}

	case "Values":
		return &schema.Type{
			Typ: &schema.Type_Config{
				Config: &schema.ConfigValue{
					IsValuesList: true,
					Elem: &schema.Type{
						Typ: &schema.Type_List{
							List: &schema.List{
								Elem: nil, // we set this to nil here, an expect the type resolver to set it later
							},
						},
					},
				},
			},
		}

	// Built-in helper types we expose in the config package
	case "Bool":
		return createBuiltinConfigWrapper(schema.Builtin_BOOL)
	case "Int8":
		return createBuiltinConfigWrapper(schema.Builtin_INT8)
	case "Int16":
		return createBuiltinConfigWrapper(schema.Builtin_INT16)
	case "Int32":
		return createBuiltinConfigWrapper(schema.Builtin_INT32)
	case "Int64":
		return createBuiltinConfigWrapper(schema.Builtin_INT64)
	case "Uint8":
		return createBuiltinConfigWrapper(schema.Builtin_UINT8)
	case "Uint16":
		return createBuiltinConfigWrapper(schema.Builtin_UINT16)
	case "Uint32":
		return createBuiltinConfigWrapper(schema.Builtin_UINT32)
	case "Uint64":
		return createBuiltinConfigWrapper(schema.Builtin_UINT64)
	case "Float32":
		return createBuiltinConfigWrapper(schema.Builtin_FLOAT32)
	case "Float64":
		return createBuiltinConfigWrapper(schema.Builtin_FLOAT64)
	case "String":
		return createBuiltinConfigWrapper(schema.Builtin_STRING)
	case "Bytes":
		return createBuiltinConfigWrapper(schema.Builtin_BYTES)
	case "Time":
		return createBuiltinConfigWrapper(schema.Builtin_TIME)
	case "UUID":
		return createBuiltinConfigWrapper(schema.Builtin_UUID)
	case "Int":
		return createBuiltinConfigWrapper(schema.Builtin_INT)
	case "Uint":
		return createBuiltinConfigWrapper(schema.Builtin_UINT)
	default:
		p.errInSrc(srcerrors.UnknownConfigWrapperType(p.fset, selector, ident))
		return nil
	}
}

func createBuiltinConfigWrapper(builtIn schema.Builtin) *schema.Type {
	return &schema.Type{
		Typ: &schema.Type_Config{
			Config: &schema.ConfigValue{
				Elem: &schema.Type{
					Typ: &schema.Type_Builtin{
						Builtin: builtIn,
					},
				},
			},
		},
	}
}

func (p *parser) validateConfigTypes() {
	// We're looking for nested `config.Value[T]`'s
	// i.e. `config.Value[config.Value[T]]` and that is now allowed.
	for _, svc := range p.svcs {
		for _, load := range svc.ConfigLoads {
			err := schema.Walk(p.decls, load.ConfigStruct.Type, func(node any) error {
				switch node := node.(type) {
				case *schema.ConfigValue:
					if node.Elem == nil {
						return srcerrors.ConfigValueTypeNotSet(p.fset, load.FuncCall)
					}

					switch elem := node.Elem.Typ.(type) {
					case *schema.Type_Config:
						if !node.IsValuesList && !elem.Config.IsValuesList {
							return srcerrors.ConfigWrapperNested(p.fset, p.schemaToAST[node], load.FuncCall)
						}
					}
				case *schema.Struct:
					if field, found := p.hasUnexportedFields[node]; found {
						return srcerrors.ConfigTypeHasUnexportFields(p.fset, load.FuncCall, field)
					}
				}
				return nil
			})
			if err != nil {
				p.errInSrc(err)
			}
		}
	}
}
