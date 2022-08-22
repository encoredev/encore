package parser

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/walker"
	"encr.dev/parser/paths"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// cacheKeyspaceConstructor describes a particular cache keyspace constructor.
type cacheKeyspaceConstructor struct {
	HasValueType      bool
	ImplicitValueType *schema.Type
}

var keyspaceConstructors = map[string]cacheKeyspaceConstructor{
	"NewStringKeyspace": {false, &schema.Type{
		Typ: &schema.Type_Builtin{Builtin: schema.Builtin_STRING},
	}},
	"NewIntKeyspace": {false, &schema.Type{
		Typ: &schema.Type_Builtin{Builtin: schema.Builtin_INT},
	}},
	"NewFloatKeyspace": {false, &schema.Type{
		Typ: &schema.Type_Builtin{Builtin: schema.Builtin_FLOAT64},
	}},
}

func init() {
	registerResource(
		est.CacheClusterResource,
		"cache cluster",
		"https://encore.dev/docs/develop/caching",
		"cache",
		"encore.dev/storage/cache",
	)

	registerResourceCreationParser(
		est.CacheClusterResource,
		"NewCluster", 0,
		(*parser).parseCacheCluster,
		locations.AllowedIn(locations.Variable).ButNotIn(locations.Function),
	)

	registerResource(
		est.CacheKeyspaceResource,
		"cache keyspace",
		"https://encore.dev/docs/develop/caching",
		"cache",
		"encore.dev/storage/cache",
		est.CacheClusterResource,
	)

	for funcName, constructor := range keyspaceConstructors {
		numTypeArgs := 1 // always key type
		if constructor.HasValueType {
			numTypeArgs++
		}

		registerResourceCreationParser(
			est.CacheKeyspaceResource,
			funcName, numTypeArgs,
			createKeyspaceParser(funcName, constructor),
			locations.AllowedIn(locations.Variable).ButNotIn(locations.Function),
		)
	}
}

func (p *parser) parseCacheCluster(file *est.File, cursor *walker.Cursor, ident *ast.Ident, callExpr *ast.CallExpr) est.Resource {
	if len(callExpr.Args) != 2 {
		p.errf(callExpr.Pos(), "cache.NewCluster requires at least one argument, the cache cluster name given as a string literal. For example `cache.New(\"my-cache\")`")
		return nil
	}

	if file.Pkg.Service == nil {
		p.errf(callExpr.Pos(), "cache.NewCluster can only be called from within Encore services. package %s is not a service",
			file.Pkg.RelPath)
		return nil
	}

	clusterName := p.parseResourceName("cache.NewCluster", "cluster name", callExpr.Args[0])
	if clusterName == "" {
		// we already reported the error inside parseResourceName
		return nil
	}

	// check the topic isn't already declared somewhere else
	for _, cluster := range p.cacheClusters {
		if strings.EqualFold(cluster.Name, clusterName) {
			p.errf(callExpr.Args[0].Pos(), "cache cluster names must be unique, \"%s\" was previously declared in %s/%s\n"+
				"\tNote: if you wish to reuse the same cache, export the original Cache object from %s and reuse it here.",
				cluster.Name, cluster.DeclFile.Pkg.Name, cluster.DeclFile.Name, cluster.DeclFile.Pkg.Name)
			return nil
		}
	}

	cluster := &est.CacheCluster{
		Name:     clusterName,
		Doc:      cursor.DocComment(),
		DeclFile: file,
		IdentAST: ident,
	}
	p.cacheClusters = append(p.cacheClusters, cluster)

	return cluster
}

func createKeyspaceParser(funcName string, constructor cacheKeyspaceConstructor) func(*parser, *est.File, *walker.Cursor, *ast.Ident, *ast.CallExpr) est.Resource {
	return func(p *parser, file *est.File, cursor *walker.Cursor, ident *ast.Ident, callExpr *ast.CallExpr) est.Resource {
		if len(callExpr.Args) != 2 {
			p.errf(
				callExpr.Pos(),
				"cache.%s requires three arguments, the topic, the subscription name given as a string literal and the subscription configuration",
				funcName,
			)
			return nil
		}

		if file.Pkg.Service == nil {
			p.errf(callExpr.Pos(), "cache.%s can only be called from within Encore services. package %s is not a service",
				funcName, file.Pkg.RelPath)
			return nil
		}

		resource := p.resourceFor(file, callExpr.Args[0])
		if resource == nil {
			p.errf(callExpr.Args[0].Pos(), "cache.%s requires the first argument to reference a cache cluster, was given %v.",
				funcName, prettyPrint(callExpr.Args[0]))
			return nil
		}
		cluster, ok := resource.(*est.CacheCluster)
		if !ok {
			p.errf(
				callExpr.Fun.Pos(),
				"cache.%s requires the first argument to reference a cache cluster, was given a %v.",
				funcName,
				reflect.TypeOf(resource),
			)
			return nil
		}

		// Parse the literal struct representing the subscription configuration
		// so we can extract the reference to the handler function
		cfg, ok := p.parseStructLit(file, "cache.KeyspaceConfig", callExpr.Args[1])
		if !ok {
			return nil
		}

		if !cfg.FullyConstant() {
			for fieldName, expr := range cfg.DynamicFields() {
				p.errf(expr.Pos(), "All values in cache.KeyspaceConfig must be a constant, however %s was not a constant, got %s", fieldName, prettyPrint(expr))
			}
			return nil
		}

		if !cfg.IsSet("Path") {
			p.errf(callExpr.Args[2].Pos(), "cache.KeyspaceConfig requires the field \"Path\" to be set")
			return nil
		}

		pathPos := cfg.Pos("Path")
		pathStr := cfg.Str("Path", "")
		if pathStr == "" {
			p.errf(pathPos, "cache.KeyspaceConfig requires the configuration field named \"Path\" to be populated with a valid keyspace path.")
			return nil
		}

		// TODO avoid requirement of leading '/'
		path, err := paths.Parse(pathPos, pathStr)
		if err != nil {
			p.errf(pathPos, "cache.KeyspaceConfig got an invalid path: %v", err)
			return nil
		}
		// TODO check for non-overlapping paths

		// Resolve key and value types
		typeArgs := getTypeArguments(callExpr.Fun)
		var keyType *schema.Type
		var valueParam *est.Param
		{
			keyType = p.resolveType(file.Pkg, file, typeArgs[0], nil)
			if _, isPtr := typeArgs[0].(*ast.StarExpr); isPtr {
				p.errf(pathPos, "cache.%s does not accept pointer types as the key type parameter, use a non-pointer type instead",
					funcName)
				return nil
			}

			if constructor.HasValueType {
				valueParam = p.resolveParameter("value type parameter", file.Pkg, file, typeArgs[1])
			} else {
				valueParam = &est.Param{IsPtr: false, Type: constructor.ImplicitValueType}
			}
		}

		keyspace := &est.CacheKeyspace{
			Cluster:   cluster,
			Doc:       cursor.DocComment(),
			DeclFile:  file,
			IdentAST:  ident,
			Path:      path,
			ConfigLit: cfg.Lit(),
			KeyType:   keyType,
			ValueType: valueParam,
		}
		p.validateCacheKeyspace(keyspace, funcName)

		cluster.Keyspaces = append(cluster.Keyspaces, keyspace)
		return keyspace
	}
}

// validateCacheKeyspace validates a parsed cache keyspace.
func (p *parser) validateCacheKeyspace(ks *est.CacheKeyspace, funcName string) {
	validateBuiltin := func(b schema.Builtin) error {
		switch b {
		case schema.Builtin_ANY:
			return fmt.Errorf("'any'/'interface{}' is not supported")
		case schema.Builtin_JSON:
			return fmt.Errorf("json.RawMessage is not supported")
		case schema.Builtin_FLOAT64, schema.Builtin_FLOAT32:
			return fmt.Errorf("floating point values are not supported")
		}
		return nil
	}

	// Check that the key type is a basic type or a named struct
	key := ks.KeyType
	switch key := key.Typ.(type) {
	case *schema.Type_Builtin:
		if err := validateBuiltin(key.Builtin); err != nil {
			p.errf(ks.Ident().Pos(), "cache.%s has invalid key type parameter: %v", funcName, err)
		}

	case *schema.Type_Named:
		named := p.decls[key.Named.Id]
		st := named.Type.GetStruct()
		if st == nil {
			p.errf(ks.Ident().Pos(), "cache.%s key type must be a basic type or a named struct type",
				funcName)
			return
		}

		// Validate struct fields
		for _, f := range st.Fields {
			switch typ := f.Typ.Typ.(type) {
			case *schema.Type_Builtin:
				if err := validateBuiltin(typ.Builtin); err != nil {
					p.errf(ks.Ident().Pos(), "cache.%s has invalid key type parameter: struct field %s is invalid: %v",
						funcName, f.Name, err)
				}

			default:
				p.errf(ks.Ident().Pos(), "cache.%s has invalid key type parameter: struct field %s is not a basic type",
					funcName, f.Name)
			}
		}
	}

	// TODO validate value type
}
