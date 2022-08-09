package parser

import (
	"go/ast"
	"reflect"
	"strings"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/walker"
	"encr.dev/parser/paths"
)

func init() {
	registerResource(
		est.CacheClusterResource,
		"cache cluster",
		"https://encore.dev/docs/develop/caching",
		"cache",
		"encore.dev/cache",
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

	type keyspaceConstructor struct {
		FuncName     string
		HasValueType bool
	}
	funcs := []keyspaceConstructor{
		{"NewStringKeyspace", false},
		{"NewIntKeyspace", false},
		{"NewFloatKeyspace", false},
	}
	for _, fn := range funcs {
		numTypeArgs := 1 // always key type
		if fn.HasValueType {
			numTypeArgs++
		}

		registerResourceCreationParser(
			est.CacheKeyspaceResource,
			fn.FuncName, numTypeArgs,
			(*parser).parseCacheKeyspace,
			locations.AllowedIn(locations.Variable).ButNotIn(locations.Function),
		)
	}
}

func (p *parser) parseCacheCluster(file *est.File, cursor *walker.Cursor, ident *ast.Ident, callExpr *ast.CallExpr) est.Resource {
	if len(callExpr.Args) != 2 {
		p.errf(callExpr.Pos(), "cache.NewCluster requires at least one argument, the cache cluster name given as a string literal. For example `cache.New(\"my-cache\")`")
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

func (p *parser) parseCacheKeyspace(file *est.File, cursor *walker.Cursor, ident *ast.Ident, callExpr *ast.CallExpr) est.Resource {
	funcName := ident.Name
	if len(callExpr.Args) != 3 {
		p.errf(
			callExpr.Pos(),
			"cache.%s requires three arguments, the topic, the subscription name given as a string literal and the subscription configuration",
			funcName,
		)
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
			reflect.TypeOf(resource),
		)
		return nil
	}

	// Parse the literal struct representing the subscription configuration
	// so we can extract the reference to the handler function
	cfg, ok := p.parseStructLit(file, "cache.KeyspaceConfig", callExpr.Args[2])
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

	// Record the subscription
	keyspace := &est.CacheKeyspace{
		Cluster:  cluster,
		Doc:      cursor.DocComment(),
		DeclFile: file,
		IdentAST: ident,
		Path:     path,
		// TODO KeyType, ValueType
	}
	cluster.Keyspaces = append(cluster.Keyspaces, keyspace)
	return keyspace
}
