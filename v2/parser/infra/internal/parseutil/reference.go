package parseutil

import (
	"go/ast"

	"encr.dev/pkg/option"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/parser/infra/internal/locations"
	"encr.dev/v2/parser/infra/resource"
)

type ReferenceSpec struct {
	AllowedLocs locations.Filter
	MinTypeArgs int
	MaxTypeArgs int
	Parse       func(ReferenceInfo)
}

type ReferenceInfo struct {
	Pass         *resource.Pass
	ResourceFunc pkginfo.QualifiedName
	File         *pkginfo.File

	Stack    []ast.Node
	Call     *ast.CallExpr
	TypeArgs []schema.Type
	Doc      string

	// Ident is the identifier this reference is assigned to, if any.
	Ident option.Option[*ast.Ident]
}

func ParseReference(p *resource.Pass, spec *ReferenceSpec, data ReferenceData) {
	selIdx := len(data.Stack) - 1
	constructor := data.ResourceFunc

	// Verify the structure of the reference.

	ident := resolveAssignedVar(data.Stack)

	// Do we have any type arguments?
	maybeHasTypeArgs := spec.MaxTypeArgs > 0

	// If we have any type arguments it will be in the parent of the selector.
	var typeArgs []schema.Type
	if maybeHasTypeArgs {
		typeArgsIdx := selIdx - 1
		if typeArgsIdx < 0 {
			p.Errs.Addf(data.Stack[selIdx].Pos(), "%s requires type arguments, but none were found",
				constructor.NaiveDisplayName())
			return
		}
		args := resolveTypeArgs(data.Stack[typeArgsIdx])
		if len(args) < spec.MinTypeArgs {
			qualifier := " at least"
			if spec.MinTypeArgs == spec.MaxTypeArgs {
				qualifier = ""
			}
			p.Errs.Addf(data.Stack[selIdx].Pos(), "%s requires%s %d type argument(s), but got %d",
				constructor.NaiveDisplayName(), qualifier, spec.MinTypeArgs, len(args))
			return
		} else if len(args) > spec.MaxTypeArgs {
			qualifier := " at most"
			if spec.MinTypeArgs == spec.MaxTypeArgs {
				qualifier = ""
			}
			p.Errs.Addf(data.Stack[selIdx].Pos(), "%s requires%s %d type argument(s), but got %d",
				constructor.NaiveDisplayName(), qualifier, spec.MaxTypeArgs, len(args))
		}
		for _, arg := range args {
			typeArgs = append(typeArgs, p.SchemaParser.ParseType(data.File, arg))
		}
	}

	// Make sure the reference is called
	callIdx := selIdx - 1
	if len(typeArgs) > 0 {
		// If there are type arguments there's an intermediary IndexExpr or IndexListExpr node.
		callIdx--
	}
	call, ok := data.Stack[callIdx].(*ast.CallExpr)
	if !ok {
		p.Errs.Addf(data.Stack[selIdx].Pos(), "%s cannot be referenced without being called",
			constructor.NaiveDisplayName())
		return
	}

	// Classify the location the current node is contained in (meaning stack[:len(stack)-1]).
	loc := locations.Classify(data.Stack[:callIdx-1])
	if !spec.AllowedLocs.Allowed(loc) {
		p.Errs.Addf(data.Stack[selIdx].Pos(), "%s cannot be called here: must be called from %s",
			constructor.NaiveDisplayName(), spec.AllowedLocs.Describe())
		return
	}

	spec.Parse(ReferenceInfo{
		Pass:         p,
		File:         data.File,
		Stack:        data.Stack,
		Ident:        ident,
		Call:         call,
		TypeArgs:     typeArgs,
		Doc:          resolveResourceDoc(data.Stack),
		ResourceFunc: data.ResourceFunc,
	})
}
