package usage

import (
	"go/ast"
	"reflect"

	"encr.dev/pkg/option"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/resource"
)

// Usage describes an infrastructure usage being used.
type Usage interface {
	usage() // marker method
	ResourceBind() resource.Bind
	ASTExpr() ast.Expr
	DeclaredIn() *pkginfo.File
}

type Base struct {
	File *pkginfo.File
	Bind resource.Bind
	Expr ast.Expr
}

func (b *Base) DeclaredIn() *pkginfo.File   { return b.File }
func (b *Base) ASTExpr() ast.Expr           { return b.Expr }
func (b *Base) ResourceBind() resource.Bind { return b.Bind }

func (b *Base) usage() {}

type Resolver struct {
	resolvers map[reflect.Type]func(*perr.List, Expr, resource.Resource) Usage
}

func (r *Resolver) Resolve(errs *perr.List, expr Expr, res resource.Resource) option.Option[Usage] {
	fn, ok := r.resolvers[reflect.TypeOf(res)]
	if !ok {
		return option.None[Usage]()
	}
	return option.AsOptional(fn(errs, expr, res))
}

func NewResolver() *Resolver {
	return &Resolver{
		resolvers: make(map[reflect.Type]func(*perr.List, Expr, resource.Resource) Usage),
	}
}

func RegisterUsageResolver[Res resource.Resource](r *Resolver, fn func(*perr.List, Expr, Res) Usage) {
	var zero Res
	typ := reflect.TypeOf(zero)

	if r.resolvers[typ] != nil {
		panic("usage resolver already registered for type " + typ.String())
	}

	r.resolvers[typ] = func(errs *perr.List, expr Expr, res resource.Resource) Usage {
		return fn(errs, expr, res.(Res))
	}
}
