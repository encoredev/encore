package parser

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"encr.dev/pkg/fns"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/scan"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/infra/cache"
	"encr.dev/v2/parser/infra/config"
	"encr.dev/v2/parser/infra/cron"
	"encr.dev/v2/parser/infra/metrics"
	"encr.dev/v2/parser/infra/pubsub"
	"encr.dev/v2/parser/infra/secrets"
	"encr.dev/v2/parser/infra/sqldb"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
	"encr.dev/v2/parser/resource/usage"
)

func NewParser(c *parsectx.Context) *Parser {
	loader := pkginfo.New(c)
	schemaParser := schema.NewParser(c, loader)
	return &Parser{
		c:            c,
		loader:       loader,
		schemaParser: schemaParser,
		registry:     resourceparser.NewRegistry(allParsers),
	}
}

type Parser struct {
	c            *parsectx.Context
	loader       *pkginfo.Loader
	schemaParser *schema.Parser
	registry     *resourceparser.Registry
}

func (p *Parser) MainModule() *pkginfo.Module {
	return p.loader.MainModule()
}

// Parse parses the given application for uses of the Encore API Framework
// and the Encore infrastructure SDK.
func (p *Parser) Parse() *Result {
	var (
		mu        sync.Mutex
		pkgs      []*pkginfo.Package
		resources []resource.Resource
		binds     []resource.Bind
	)

	scan.ProcessModule(p.c.Errs, p.loader, p.c.MainModuleDir, func(pkg *pkginfo.Package) {
		pass := &resourceparser.Pass{
			Context:      p.c,
			SchemaParser: p.schemaParser,
			Pkg:          pkg,
		}

		interested := p.registry.InterestedParsers(pkg)
		for _, p := range interested {
			p.Run(pass)
		}

		mu.Lock()
		pkgs = append(pkgs, pkg)
		resources = append(resources, pass.Resources()...)
		binds = append(binds, pass.Binds()...)
		mu.Unlock()
	})

	usages := usage.Parse(p.c.Errs, pkgs, binds)
	return computeResult(p.c.Errs, pkgs, resources, binds, usages)
}

// allParsers are all the resource parsers we support.
var allParsers = []*resourceparser.Parser{
	apis.Parser,
	cache.ClusterParser,
	cache.KeyspaceParser,
	config.LoadParser,
	cron.JobParser,
	metrics.MetricParser,
	pubsub.TopicParser,
	pubsub.SubscriptionParser,
	secrets.SecretsParser,
	sqldb.DatabaseParser,
	sqldb.NamedParser,
}

// computeResult computes the combined resource description.
func computeResult(errs *perr.List, appPackages []*pkginfo.Package, resources []resource.Resource, binds []resource.Bind, usages []usage.Usage) *Result {
	d := &Result{
		appPackages: appPackages,
		resources:   resources,
		allBinds:    binds,
		allUsages:   usages,

		resMap: make(map[resource.Resource]*resourceMeta),
		byType: make(map[reflect.Type][]resource.Resource),
	}
	d.initResources()
	d.initBinds(errs, binds)
	d.initUsages(errs, usages)
	return d
}

func Resources[R resource.Resource](res *Result) []R {
	var zero R
	resources := res.byType[reflect.TypeOf(zero)]
	return fns.Map(resources, func(r resource.Resource) R {
		return r.(R)
	})
}

type Result struct {
	appPackages []*pkginfo.Package
	resources   []resource.Resource
	allBinds    []resource.Bind
	allUsages   []usage.Usage

	resMap map[resource.Resource]*resourceMeta
	byType map[reflect.Type][]resource.Resource
}

func (d *Result) AppPackages() []*pkginfo.Package {
	return d.appPackages
}

func (d *Result) Resources() []resource.Resource {
	return d.resources
}

func (d *Result) AllBinds() []resource.Bind {
	return d.allBinds
}

func (d *Result) AllUsages() []usage.Usage {
	return d.allUsages
}

func (d *Result) Binds(res resource.Resource) []resource.Bind {
	return d.rd(res).binds
}

func (d *Result) PkgDeclBinds(res resource.Resource) []*resource.PkgDeclBind {
	return d.rd(res).pkgDecls
}

func (d *Result) Usages(res resource.Resource) []usage.Usage {
	return d.rd(res).usages
}

func (d *Result) rd(res resource.Resource) *resourceMeta {
	m := d.resMap[res]
	if m == nil {
		m = &resourceMeta{}
		d.resMap[res] = m
	}
	return m
}

// resourceMeta describes metadata about a resource.
type resourceMeta struct {
	binds    []resource.Bind
	pkgDecls []*resource.PkgDeclBind
	usages   []usage.Usage
}

func (m *resourceMeta) addBind(b resource.Bind) {
	m.binds = append(m.binds, b)
	if pkgDecl, ok := b.(*resource.PkgDeclBind); ok {
		m.pkgDecls = append(m.pkgDecls, pkgDecl)
	}
}

func (m *resourceMeta) addUsage(u usage.Usage) {
	m.usages = append(m.usages, u)
}

func (d *Result) initResources() {
	for _, res := range d.resources {
		typ := reflect.TypeOf(res)
		d.byType[typ] = append(d.byType[typ], res)
	}
}

func (d *Result) initBinds(errs *perr.List, binds []resource.Bind) {
	byPath := make(map[string]resource.Resource, len(d.resources))

	for _, r := range d.resources {
		// If we have a named resource, add it to the path map.
		if named, ok := r.(resource.Named); ok {
			p := resource.Path{{named.Kind(), named.ResourceName()}}
			byPath[pathKey(p)] = r
		}
	}

	for _, b := range binds {
		// Do we have a specific resource reference?
		ref := b.ResourceRef()
		if r := ref.Resource; r != nil {
			d.rd(r).addBind(b)
			continue
		}

		// Otherwise figure out the resource from the bind path.
		key := pathKey(ref.Path)
		if r, ok := byPath[key]; ok {
			d.rd(r).addBind(b)
		} else {
			// NOTE(andre): We could end up here in the future when we support
			// named references to PubSub subscriptions, since those would
			// involve a two-segment resource path (first the topic and then the subscription),
			// which we don't support today (the construction of byPath above only handles
			// the case of single-segment resource paths).
			// Since we don't support that today, this is fine for now.
			errs.Addf(b.Pos(), "internal compiler error: unknown resource (path %q)", key)
		}
	}
}

func (d *Result) initUsages(errs *perr.List, usages []usage.Usage) {
	resourcesByBindName := make(map[pkginfo.QualifiedName]resource.Resource, len(d.resources))
	for r, m := range d.resMap {
		for _, pkgDecl := range m.pkgDecls {
			resourcesByBindName[pkgDecl.QualifiedName()] = r
		}
	}

	for _, u := range usages {
		bind := u.ResourceBind()
		ref := bind.ResourceRef()
		if r := ref.Resource; r != nil {
			d.rd(r).addUsage(u)
		} else if pkgDecl, ok := bind.(*resource.PkgDeclBind); ok {
			if r, ok := resourcesByBindName[pkgDecl.QualifiedName()]; ok {
				d.rd(r).addUsage(u)
			} else {
				errs.Addf(u.ASTExpr().Pos(), "internal compiler error: resource reference not found: %s",
					pkgDecl.QualifiedName().NaiveDisplayName())
			}
		} else {
			errs.Addf(u.ASTExpr().Pos(), "internal compiler error: invalid resource bind: %T", bind)
		}
	}
}

func pathKey(path resource.Path) string {
	var b strings.Builder
	for i, e := range path {
		if i > 0 {
			b.WriteString("/")
		}
		fmt.Fprintf(&b, "%s:%s", e.Kind.String(), e.Name)
	}
	return b.String()
}
