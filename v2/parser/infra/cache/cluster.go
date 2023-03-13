package cache

import (
	"go/ast"

	"encore.dev/storage/cache"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	literals "encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/locations"
	parseutil "encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/resource"
)

type Cluster struct {
	AST            *ast.CallExpr
	Name           string // The unique name of the cache cluster
	Doc            string // The documentation on the cluster
	EvictionPolicy string
	File           *pkginfo.File
}

func (c *Cluster) Kind() resource.Kind       { return resource.CacheCluster }
func (c *Cluster) Package() *pkginfo.Package { return c.File.Pkg }
func (c *Cluster) ASTExpr() ast.Expr         { return c.AST }
func (c *Cluster) ResourceName() string      { return c.Name }

var ClusterParser = &resource.Parser{
	Name: "Cache Cluster",

	InterestingImports: []paths.Pkg{"encore.dev/storage/cache"},
	Run: func(p *resource.Pass) {
		name := pkginfo.QualifiedName{PkgPath: "encore.dev/storage/cache", Name: "NewCluster"}

		spec := &parseutil.ReferenceSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 0,
			MaxTypeArgs: 0,
			Parse:       parseCluster,
		}

		parseutil.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			parseutil.ParseReference(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
		})
	},
}

func parseCluster(d parseutil.ReferenceInfo) {
	displayName := d.ResourceFunc.NaiveDisplayName()
	if len(d.Call.Args) != 2 {
		d.Pass.Errs.Add(errExpectsTwoArgs(displayName).AtGoNode(d.Call))
		return
	}

	clusterName := parseutil.ParseResourceName(d.Pass.Errs, displayName, "cache cluster name",
		d.Call.Args[0], parseutil.KebabName, "")
	if clusterName == "" {
		// we already reported the error inside ParseResourceName
		return
	}

	cfgLit, ok := literals.ParseStruct(d.Pass.Errs, d.File, "cache.ClusterConfig", d.Call.Args[1])
	if !ok {
		return // error reported by ParseStruct
	}

	// Decode the config
	type decodedConfig struct {
		EvictionPolicy string   `literal:",optional"`
		DefaultExpiry  ast.Expr `literal:",optional,dynamic"`
	}
	config := literals.Decode[decodedConfig](d.Pass.Errs, cfgLit)

	if config.EvictionPolicy == "" {
		config.EvictionPolicy = string(cache.AllKeysLRU)
	}

	switch cache.EvictionPolicy(config.EvictionPolicy) {
	case cache.AllKeysLRU, cache.AllKeysLFU, cache.AllKeysRandom, cache.VolatileLRU,
		cache.VolatileLFU, cache.VolatileTTL, cache.VolatileRandom, cache.NoEviction:
		// all good
	default:
		d.Pass.Errs.Add(errInvalidEvictionPolicy.AtGoNode(d.Call.Args[1]))
		return
	}

	cluster := &Cluster{
		AST:            d.Call,
		Name:           clusterName,
		Doc:            d.Doc,
		EvictionPolicy: config.EvictionPolicy,
		File:           d.File,
	}

	d.Pass.RegisterResource(cluster)
	if id, ok := d.Ident.Get(); ok {
		d.Pass.AddBind(id, cluster)
	}
}
