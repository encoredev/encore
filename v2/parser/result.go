package parser

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"

	"encr.dev/internal/paths"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/posmap"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/usage"
)

// computeResult computes the combined resource description.
func computeResult(errs *perr.List, mainModule *pkginfo.Module, ur *usage.Resolver, appPackages []*pkginfo.Package, resources []resource.Resource, binds []resource.Bind, usageExprs []usage.Expr) *Result {
	d := &Result{
		mainModule:    mainModule,
		appPackages:   appPackages,
		resources:     resources,
		allBinds:      binds,
		allUsageExprs: usageExprs,

		resMap:         make(map[resource.Resource]*resourceMeta),
		byType:         make(map[reflect.Type][]resource.Resource),
		bindToResource: make(map[resource.Bind]resource.Resource, len(binds)),
		qnToBind:       make(map[pkginfo.QualifiedName]resource.Bind, len(binds)),
		usageByPkg:     make(map[paths.Pkg][]usage.Usage),
	}

	d.initResources()
	d.initBinds(errs, binds)
	d.initUsages(errs, ur, usageExprs)
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
	mainModule    *pkginfo.Module
	appPackages   []*pkginfo.Package
	resources     []resource.Resource
	allBinds      []resource.Bind
	allUsageExprs []usage.Expr

	resMap         map[resource.Resource]*resourceMeta
	byType         map[reflect.Type][]resource.Resource
	bindToResource map[resource.Bind]resource.Resource
	qnToBind       map[pkginfo.QualifiedName]resource.Bind
	resByPos       posmap.Map[resource.Resource]
	usageByPos     posmap.Map[usage.Usage]
	usageByPkg     map[paths.Pkg][]usage.Usage
}

func (d *Result) MainModule() *pkginfo.Module {
	return d.mainModule
}

func (d *Result) AppPackages() []*pkginfo.Package {
	return d.appPackages
}

func (d *Result) PackageAt(path paths.Pkg) option.Option[*pkginfo.Package] {
	for _, pkg := range d.appPackages {
		if pkg.ImportPath == path {
			return option.Some(pkg)
		}
	}

	return option.None[*pkginfo.Package]()
}

func (d *Result) Resources() []resource.Resource {
	return d.resources
}

func (d *Result) AllBinds() []resource.Bind {
	return d.allBinds
}

func (d *Result) AllUsageExprs() []usage.Expr {
	return d.allUsageExprs
}

func (d *Result) ResourceForBind(b resource.Bind) resource.Resource {
	return d.bindToResource[b]
}

func (d *Result) ResourceForQN(qn pkginfo.QualifiedName) option.Option[resource.Resource] {
	return option.Map(d.BindForQN(qn), d.ResourceForBind)
}

func (d *Result) BindForQN(qn pkginfo.QualifiedName) option.Option[resource.Bind] {
	return option.AsOptional(d.qnToBind[qn])
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

func (d *Result) AllUsages() []usage.Usage {
	var all []usage.Usage
	for _, res := range d.resources {
		all = append(all, d.Usages(res)...)
	}
	return all
}

func (d *Result) ResourceConstructorContaining(node ast.Node) option.Option[resource.Resource] {
	return d.resByPos.Containing(node)
}

func (d *Result) UsageFromNode(node ast.Node) option.Option[usage.Usage] {
	return d.usageByPos.Containing(node)
}

func (d *Result) UsagesInPkg(pkgPath paths.Pkg) []usage.Usage {
	return d.usageByPkg[pkgPath]
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
	d.resByPos = posmap.Build(d.resources...)

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

	addBind := func(r resource.Resource, b resource.Bind) {
		d.rd(r).addBind(b)
		d.bindToResource[b] = r
		if pkgBind, ok := b.(*resource.PkgDeclBind); ok && pkgBind != nil {
			d.qnToBind[pkgBind.QualifiedName()] = b
		}
	}

	for _, b := range binds {
		// Do we have a specific resource reference?
		ref := b.ResourceRef()
		if r := ref.Resource; r != nil {
			addBind(r, b)
			continue
		}

		// Otherwise figure out the resource from the bind path.
		key := pathKey(ref.Path)
		if r, ok := byPath[key]; ok {
			addBind(r, b)
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

func (d *Result) initUsages(errs *perr.List, ur *usage.Resolver, usageExprs []usage.Expr) {
	resourcesByBindName := make(map[pkginfo.QualifiedName]resource.Resource, len(d.resources))
	for r, m := range d.resMap {
		for _, pkgDecl := range m.pkgDecls {
			resourcesByBindName[pkgDecl.QualifiedName()] = r
		}
	}

	var allUsages []usage.Usage

	processUsage := func(r resource.Resource, expr usage.Expr) {
		if u, ok := ur.Resolve(errs, expr, r).Get(); ok {
			d.rd(r).addUsage(u)
			allUsages = append(allUsages, u)

			pkgPath := u.DeclaredIn().Pkg.ImportPath
			d.usageByPkg[pkgPath] = append(d.usageByPkg[pkgPath], u)
		}
	}

	for _, u := range usageExprs {
		bind := u.ResourceBind()
		ref := bind.ResourceRef()
		if r := ref.Resource; r != nil {
			processUsage(r, u)
		} else if pkgDecl, ok := bind.(*resource.PkgDeclBind); ok {
			if r, ok := resourcesByBindName[pkgDecl.QualifiedName()]; ok {
				processUsage(r, u)
			} else {
				errs.Addf(u.ASTExpr().Pos(), "internal compiler error: resource reference not found: %s",
					pkgDecl.QualifiedName().NaiveDisplayName())
			}
		} else {
			errs.Addf(u.ASTExpr().Pos(), "internal compiler error: invalid resource bind: %T", bind)
		}
	}

	d.usageByPos = posmap.Build[usage.Usage](allUsages...)
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
