package parser

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"

	"encore.dev/storage/cache"
	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/walker"
	"encr.dev/parser/paths"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

type valueKind int

const (
	// implicitValue means the user does not specify the value type as
	// a type parameter, but is implicit from the constructor name.
	// If used, the ImplicitValueType field must be set on the constructor spec.
	implicitValue valueKind = iota

	// basicValue means the constructor supports basic types only.
	basicValue

	// structValue means the constructor supports struct values only.
	structValue
)

// cacheKeyspaceConstructor describes a particular cache keyspace constructor.
type cacheKeyspaceConstructor struct {
	FuncName          string
	ValueKind         valueKind
	ImplicitValueType *schema.Type
}

var keyspaceConstructors = []cacheKeyspaceConstructor{
	{"NewStringKeyspace", implicitValue, &schema.Type{
		Typ: &schema.Type_Builtin{Builtin: schema.Builtin_STRING},
	}},
	{"NewIntKeyspace", implicitValue, &schema.Type{
		Typ: &schema.Type_Builtin{Builtin: schema.Builtin_INT64},
	}},
	{"NewFloatKeyspace", implicitValue, &schema.Type{
		Typ: &schema.Type_Builtin{Builtin: schema.Builtin_FLOAT64},
	}},
	{"NewListKeyspace", basicValue, nil},
	{"NewSetKeyspace", basicValue, nil},
	{"NewStructKeyspace", structValue, nil},
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

	for _, constructor := range keyspaceConstructors {
		numTypeArgs := 1 // always key type
		if constructor.ValueKind != implicitValue {
			numTypeArgs++
		}

		registerResourceCreationParser(
			est.CacheKeyspaceResource,
			constructor.FuncName, numTypeArgs,
			createKeyspaceParser(constructor),
			locations.AllowedIn(locations.Variable).ButNotIn(locations.Function),
		)
	}
}

func (p *parser) parseCacheCluster(file *est.File, cursor *walker.Cursor, ident *ast.Ident, callExpr *ast.CallExpr) est.Resource {
	if len(callExpr.Args) != 2 {
		p.errf(callExpr.Pos(), "cache.NewCluster requires at least two arguments, the cache cluster name given as a string literal and a cluster config.`")
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
				"\tNote: if you wish to reuse the same cluster, export the original cache.Cluster object from %s and reuse it here.",
				cluster.Name, cluster.DeclFile.Pkg.Name, cluster.DeclFile.Name, cluster.DeclFile.Pkg.Name)
			return nil
		}
	}

	// Parse the literal struct representing the cluster configuration.
	cfg, ok := p.parseStructLit(file, "cache.ClusterConfig", callExpr.Args[1])
	if !ok {
		return nil
	}

	if !cfg.FullyConstant() {
		dynamic := cfg.DynamicFields()
		failed := false
		for fieldName, expr := range dynamic {
			switch fieldName {
			case "DefaultExpiry":
				// allowed
			default:
				failed = true
				p.errf(expr.Pos(), "The %s field in cache.KeyspaceConfig must be a constant literal, got %v", fieldName, prettyPrint(expr))
			}
		}
		if failed {
			return nil
		}
	}

	evictionPolicy := cfg.Str("EvictionPolicy", string(cache.AllKeysLRU))
	switch cache.EvictionPolicy(evictionPolicy) {
	case cache.AllKeysLRU, cache.AllKeysLFU, cache.AllKeysRandom, cache.VolatileLRU,
		cache.VolatileLFU, cache.VolatileTTL, cache.VolatileRandom, cache.NoEviction:
		// all good
	default:
		p.errf(callExpr.Args[1].Pos(), "invalid \"EvictionPolicy\" value: %q", evictionPolicy)
		return nil
	}

	cluster := &est.CacheCluster{
		Name:           clusterName,
		Doc:            cursor.DocComment(),
		DeclFile:       file,
		IdentAST:       ident,
		EvictionPolicy: evictionPolicy,
	}
	p.cacheClusters = append(p.cacheClusters, cluster)

	return cluster
}

func createKeyspaceParser(con cacheKeyspaceConstructor) func(*parser, *est.File, *walker.Cursor, *ast.Ident, *ast.CallExpr) est.Resource {
	return func(p *parser, file *est.File, cursor *walker.Cursor, ident *ast.Ident, callExpr *ast.CallExpr) est.Resource {
		if len(callExpr.Args) != 2 {
			p.errf(
				callExpr.Pos(),
				"cache.%s requires two arguments: the cache cluster and the keyspace configuration",
				con.FuncName,
			)
			return nil
		}

		svc := file.Pkg.Service
		if svc == nil {
			p.errf(callExpr.Pos(), "cache.%s can only be called from within Encore services. package %s is not a service",
				con.FuncName, file.Pkg.RelPath)
			return nil
		}

		resource := p.resourceFor(file, callExpr.Args[0])
		if resource == nil {
			p.errf(callExpr.Args[0].Pos(), "cache.%s requires the first argument to reference a cache cluster, was given %v.",
				con.FuncName, prettyPrint(callExpr.Args[0]))
			return nil
		}
		cluster, ok := resource.(*est.CacheCluster)
		if !ok {
			p.errf(
				callExpr.Fun.Pos(),
				"cache.%s requires the first argument to reference a cache cluster, was given a %v.",
				con.FuncName,
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
			dynamic := cfg.DynamicFields()
			failed := false
			for fieldName, expr := range dynamic {
				switch fieldName {
				case "DefaultExpiry":
					// allowed
				default:
					failed = true
					p.errf(expr.Pos(), "The %s field in cache.KeyspaceConfig must be a constant literal, got %v", fieldName, prettyPrint(expr))
				}
			}
			if failed {
				return nil
			}
		}

		if !cfg.IsSet("KeyPattern") {
			p.errf(callExpr.Args[1].Pos(), "cache.KeyspaceConfig requires the field \"KeyPattern\" to be set")
			return nil
		}

		keyPatternPos := cfg.Pos("KeyPattern")
		keyPatternStr := cfg.Str("KeyPattern", "")
		if keyPatternStr == "" {
			p.errf(keyPatternPos, "cache.KeyspaceConfig requires the configuration field named \"KeyPattern\" to be populated with a valid keyspace pattern.")
			return nil
		}

		const reservedPrefix = "__encore"
		if strings.HasPrefix(keyPatternStr, reservedPrefix) {
			p.errf(keyPatternPos, "invalid KeyPattern: use of reserved prefix %q", reservedPrefix)
			return nil
		}

		path, err := paths.Parse(keyPatternPos, keyPatternStr, paths.CacheKeyspace)
		if err != nil {
			p.errf(keyPatternPos, "cache.KeyspaceConfig got an invalid keyspace pattern: %v", err)
			return nil
		}

		// Resolve key and value types
		typeArgs := getTypeArguments(callExpr.Fun)
		var keyType, valueType *schema.Type
		{
			keyType = p.resolveType(file.Pkg, file, typeArgs[0], nil)
			if _, isPtr := typeArgs[0].(*ast.StarExpr); isPtr {
				p.errf(keyPatternPos, "cache.%s does not accept pointer types as the key type parameter, use a non-pointer type instead",
					con.FuncName)
				return nil
			}

			if con.ValueKind == implicitValue {
				valueType = con.ImplicitValueType
			} else {
				valueType = p.resolveType(file.Pkg, file, typeArgs[1], nil)
			}
		}

		keyspace := &est.CacheKeyspace{
			Cluster:   cluster,
			Svc:       svc,
			Doc:       cursor.DocComment(),
			DeclFile:  file,
			IdentAST:  ident,
			Path:      path,
			ConfigLit: cfg.Lit(),
			KeyType:   keyType,
			ValueType: valueType,
		}
		p.validateCacheKeyspace(keyspace, con)

		cluster.Keyspaces = append(cluster.Keyspaces, keyspace)
		return keyspace
	}
}

// validateCacheKeyspace validates a parsed cache keyspace.
func (p *parser) validateCacheKeyspace(ks *est.CacheKeyspace, con cacheKeyspaceConstructor) {
	// Check that the key type is a basic type or a named struct
	{
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

		key := ks.KeyType
		_, keyIsBuiltin := key.Typ.(*schema.Type_Builtin)

		seenPathSegments := make(map[string]bool)
		for _, seg := range ks.Path.Segments {
			if seg.Type == paths.Literal {
				continue
			}
			name := seg.Value
			seenPathSegments[name] = false

			if keyIsBuiltin && name != "key" {
				p.errf(ks.Ident().Pos(), "KeyPattern parameter must be named ':key' for basic (non-struct) key types")
			}
		}

		switch key := key.Typ.(type) {
		case *schema.Type_Builtin:
			if err := validateBuiltin(key.Builtin); err != nil {
				p.errf(ks.Ident().Pos(), "cache.%s has invalid key type parameter: %v", con.FuncName, err)
			}

		case *schema.Type_Named:
			named := p.decls[key.Named.Id]
			st := named.Type.GetStruct()
			if st == nil {
				p.errf(ks.Ident().Pos(), "cache.%s key type must be a basic type or a named struct type",
					con.FuncName)
				return
			}

			// Validate struct fields
			for _, f := range st.Fields {
				fieldName := f.Name
				if _, exists := seenPathSegments[fieldName]; !exists {
					p.errf(ks.Ident().Pos(), "invalid cache value type %s: field %s not used in KeyPattern",
						named.Name, fieldName)
				} else {
					seenPathSegments[fieldName] = true
				}

				switch typ := f.Typ.Typ.(type) {
				case *schema.Type_Builtin:
					if err := validateBuiltin(typ.Builtin); err != nil {
						p.errf(ks.Ident().Pos(), "cache.%s has invalid key type parameter: struct field %s is invalid: %v",
							con.FuncName, fieldName, err)
					}

				default:
					p.errf(ks.Ident().Pos(), "cache.%s has invalid key type parameter: struct field %s is not a basic type",
						con.FuncName, fieldName)
				}
			}

			// Ensure all path segments are valid field names
			for fieldName, seen := range seenPathSegments {
				if !seen {
					p.errf(ks.Ident().Pos(), "invalid cache KeyPattern: field %s does not exist in value type %s",
						fieldName, named.Name)
				}
			}
		}
	}

	// Check the value type. We only need to do this for struct types since they need
	// to be represented as 'any' constraints. Basic type constructors enforce that the value type
	// through the Go type system and don't need to be verified again.
	if con.ValueKind == structValue {
		value := ks.ValueType
		ok := false
		if named := value.GetNamed(); named != nil {
			if st := p.decls[named.Id].Type.GetStruct(); st != nil {
				ok = true
			}
		}
		if !ok {
			p.errf(ks.Ident().Pos(), "cache.%s has invalid value type parameter: must be a named struct type",
				con.FuncName)
		}
	}
}

func (p *parser) validateCacheKeyspacePathConflicts() {
	for _, cc := range p.cacheClusters {
		var set paths.Set
		pathToKeyspace := make(map[*paths.Path]*est.CacheKeyspace)
		for _, ks := range cc.Keyspaces {
			pathToKeyspace[ks.Path] = ks

			if err := set.Add("", ks.Path); err != nil {
				conflict := err.(*paths.ConflictError)
				other := pathToKeyspace[conflict.Other]
				p.errf(ks.Ident().Pos(), "cache KeyPattern conflict: %v\n\tsee other cache declaration at %s",
					err, p.fset.Position(other.Ident().Pos()))
			}
		}
	}
}
