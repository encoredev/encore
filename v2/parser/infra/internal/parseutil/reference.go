package parseutil

import (
	"go/ast"

	"encr.dev/pkg/option"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/parser/infra/internal/locations"
	"encr.dev/v2/parser/resource/resourceparser"
)

type ReferenceSpec struct {
	MinTypeArgs int
	MaxTypeArgs int
	Parse       func(ReferenceInfo)
}

type ReferenceInfo struct {
	Pass         *resourceparser.Pass
	ResourceFunc pkginfo.QualifiedName
	File         *pkginfo.File

	Stack    []ast.Node
	Call     *ast.CallExpr
	TypeArgs []schema.Type
	Doc      string

	// Ident is the identifier this reference is assigned to, if any.
	Ident option.Option[*ast.Ident]
}

type ReferenceData struct {
	File         *pkginfo.File
	Stack        []ast.Node
	ResourceFunc pkginfo.QualifiedName
}

func ParseReference(p *resourceparser.Pass, spec *ReferenceSpec, data ReferenceData) {
	selIdx := len(data.Stack) - 1
	constructor := data.ResourceFunc

	// Do we have any type arguments?
	maybeHasTypeArgs := spec.MaxTypeArgs > 0

	// If we have any type arguments it will be in the parent of the selector.
	var typeArgs []schema.Type
	if maybeHasTypeArgs {
		typeArgsIdx := selIdx - 1
		if typeArgsIdx < 0 {
			p.Errs.Add(errRequiresTypeArgumentsNoneFound(constructor.NaiveDisplayName()).AtGoNode(data.Stack[selIdx]))
			return
		}
		args := resolveTypeArgs(data.Stack[typeArgsIdx])
		if len(args) < spec.MinTypeArgs {
			qualifier := " at least"
			if spec.MinTypeArgs == spec.MaxTypeArgs {
				qualifier = ""
			}

			p.Errs.Add(errWrongNumberOfTypeArguments(constructor.NaiveDisplayName(), qualifier, spec.MinTypeArgs, len(args)).AtGoNode(data.Stack[selIdx]))
			return
		} else if len(args) > spec.MaxTypeArgs {
			qualifier := " at most"
			if spec.MinTypeArgs == spec.MaxTypeArgs {
				qualifier = ""
			}
			p.Errs.Add(errWrongNumberOfTypeArguments(constructor.NaiveDisplayName(), qualifier, spec.MaxTypeArgs, len(args)).AtGoNode(data.Stack[selIdx]))
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
		p.Errs.Add(errCannotBeReferencedWithoutBeingCalled(constructor.NaiveDisplayName()).AtGoNode(data.Stack[selIdx]))
		return
	}

	// Classify the location of the current node
	cls := locations.Classify(data.Stack[:callIdx+1])

	var pkgIdent *ast.Ident
	switch cls := cls.(type) {
	case locations.PkgVar:
		pkgIdent = cls.Ident
	case locations.OtherPkgExpr:
		// Allowed; not assigned to a variable
	default:
		// Everything else is disallowed
		p.Errs.Add(errCannotBeCalledHere(constructor.NaiveDisplayName(), "a package level variable").AtGoNode(data.Stack[selIdx]))
		return
	}

	spec.Parse(ReferenceInfo{
		Pass:         p,
		File:         data.File,
		Stack:        data.Stack,
		Ident:        option.AsOptional(pkgIdent),
		Call:         call,
		TypeArgs:     typeArgs,
		Doc:          resolveResourceDoc(data.Stack),
		ResourceFunc: data.ResourceFunc,
	})
}
