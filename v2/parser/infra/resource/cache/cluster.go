package cache

import (
	"go/ast"

	"encore.dev/storage/cache"
	"encr.dev/pkg/option"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	literals "encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/locations"
	parseutil "encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/infra/resource"
)

type Cluster struct {
	AST            *ast.CallExpr
	Name           string     // The unique name of the cache cluster
	Ident          *ast.Ident // The identifier of the cache cluster
	Doc            string     // The documentation on the cluster
	EvictionPolicy string
	File           *pkginfo.File
}

func (c *Cluster) Kind() resource.Kind       { return resource.CacheCluster }
func (c *Cluster) DeclaredIn() *pkginfo.File { return c.File }
func (c *Cluster) ASTExpr() ast.Expr         { return c.AST }
func (c *Cluster) BoundTo() option.Option[pkginfo.QualifiedName] {
	return parseutil.BoundTo(c.File, c.Ident)
}

var ClusterParser = &resource.Parser{
	Name: "Cache Cluster",

	RequiredImports: []paths.Pkg{"encore.dev/storage/cache"},
	Run: func(p *resource.Pass) []resource.Resource {
		name := pkginfo.QualifiedName{PkgPath: "encore.dev/storage/cache", Name: "NewCluster"}

		spec := &parseutil.ResourceCreationSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 0,
			MaxTypeArgs: 0,
			Parse:       parseCluster,
		}

		var resources []resource.Resource
		parseutil.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			r := parseutil.ParseResourceCreation(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
			if r != nil {
				resources = append(resources, r)
			}
		})
		return resources
	},
}

func parseCluster(d parseutil.ParseData) resource.Resource {
	displayName := d.ResourceFunc.NaiveDisplayName()
	if len(d.Call.Args) != 2 {
		d.Pass.Errs.Addf(d.Call.Pos(), "%s expects 2 arguments", displayName)
		return nil
	}

	clusterName := parseutil.ParseResourceName(d.Pass.Errs, displayName, "cache cluster name",
		d.Call.Args[0], parseutil.KebabName, "")
	if clusterName == "" {
		// we already reported the error inside ParseResourceName
		return nil
	}

	cfgLit, ok := literals.ParseStruct(d.Pass.Errs, d.File, "cache.ClusterConfig", d.Call.Args[1])
	if !ok {
		return nil // error reported by ParseStruct
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
		d.Pass.Errs.Addf(d.Call.Args[1].Pos(), "invalid \"EvictionPolicy\" value: %q", config.EvictionPolicy)
		return nil
	}

	return &Cluster{
		AST:            d.Call,
		Name:           clusterName,
		Ident:          d.Ident,
		Doc:            d.Doc,
		EvictionPolicy: config.EvictionPolicy,
		File:           d.File,
	}
}
