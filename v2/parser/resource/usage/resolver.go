package usage

import (
	"go/ast"
	"go/token"

	"encr.dev/pkg/option"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/resource"
)

// Usage describes an infrastructure usage being used.
type Usage interface {
	ast.Node

	usage() // marker method
	ResourceBind() resource.Bind
	ASTExpr() ast.Expr
	DeclaredIn() *pkginfo.File
}

type Base struct {
	File *pkginfo.File
	Bind resource.Bind
	Expr Expr
}

func (b *Base) DeclaredIn() *pkginfo.File   { return b.File }
func (b *Base) ASTExpr() ast.Expr           { return b.Expr.ASTExpr() }
func (b *Base) ResourceBind() resource.Bind { return b.Bind }
func (b *Base) Pos() token.Pos              { return b.Expr.Pos() }
func (b *Base) End() token.Pos              { return b.Expr.End() }

func (b *Base) usage() {}

type Resolver struct {
	resolvers map[resource.Kind]func(ResolveData, resource.Resource) Usage
}

func (r *Resolver) Resolve(errs *perr.List, expr Expr, res resource.Resource) option.Option[Usage] {
	fn, ok := r.resolvers[res.Kind()]
	if !ok {
		return option.None[Usage]()
	}
	data := ResolveData{
		Errs: errs,
		Expr: expr,
	}
	return option.AsOptional(fn(data, res))
}

func NewResolver() *Resolver {
	return &Resolver{
		resolvers: make(map[resource.Kind]func(ResolveData, resource.Resource) Usage),
	}
}

func RegisterUsageResolver[Res resource.Resource](r *Resolver, fn func(ResolveData, Res) Usage) {
	var zero Res

	if r.resolvers[zero.Kind()] != nil {
		panic("usage resolver already registered for type " + zero.Kind().String())
	}

	r.resolvers[zero.Kind()] = func(data ResolveData, res resource.Resource) Usage {
		return fn(data, res.(Res))
	}
}
