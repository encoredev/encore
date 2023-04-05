package pubsub

import (
	"encr.dev/pkg/option"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/resource/usage"
)

type PublishUsage struct {
	usage.Base
}

type RefUsage struct {
	usage.Base
	Perms []Perm
}

func (u *RefUsage) HasPerm(perm Perm) bool {
	for _, p := range u.Perms {
		if p == perm {
			return true
		}
	}
	return false
}

type Perm string

const (
	PublishPerm Perm = "publish"
)

func ResolveTopicUsage(data usage.ResolveData, topic *Topic) usage.Usage {
	switch expr := data.Expr.(type) {
	case *usage.MethodCall:
		if expr.Method == "Publish" {
			return &PublishUsage{
				Base: usage.Base{
					File: expr.File,
					Bind: expr.Bind,
					Expr: expr,
				},
			}
		} else {
			return nil
		}

	case *usage.FuncArg:
		switch {
		case option.Contains(expr.PkgFunc, pkginfo.Q("encore.dev/pubsub", "NewSubscription")):
			// Allowed usage
			return nil
		case option.Contains(expr.PkgFunc, pkginfo.Q("encore.dev/et", "Topic")):
			// Allowed usage
			return nil
		case option.Contains(expr.PkgFunc, pkginfo.Q("encore.dev/pubsub", "TopicRef")):
			return parseTopicRef(data.Errs, expr)
		}
	}

	data.Errs.Add(errInvalidTopicUsage.AtGoNode(data.Expr))
	return nil
}

func parseTopicRef(errs *perr.List, expr *usage.FuncArg) usage.Usage {
	if len(expr.TypeArgs) < 1 {
		errs.Add(errTopicRefNoTypeArgs.AtGoNode(expr.Call))
		return nil
	}

	checkUsage := func(typ schema.Type) (usage.Usage, bool) {
		if schemautil.IsNamed(typ, "encore.dev/pubsub", "Publisher") {
			return &RefUsage{
				Base: usage.Base{
					File: expr.File,
					Bind: expr.Bind,
					Expr: expr,
				},
				Perms: []Perm{PublishPerm},
			}, true
		}
		return nil, false
	}

	// Do we have a simple usage directly as the type argument?
	if u, ok := checkUsage(expr.TypeArgs[0]); ok {
		return u
	}

	// Determine if we have a custom ref type,
	// either in the form "type Foo = pubsub.Publisher[Msg]"
	// or in the form "type Foo interface { pubsub.Publisher[Msg] }"
	if named, ok := expr.TypeArgs[0].(schema.NamedType); ok {
		underlying := named.Decl().Type
		if u, ok := checkUsage(underlying); ok {
			return u
		}

		// Otherwise make sure the interface only embeds the one supported type we have (pubsub.Publisher).
		// We'll need to extend this in the future to support multiple permissions.
		if iface, ok := underlying.(schema.InterfaceType); ok {
			if len(iface.EmbeddedIfaces) == 1 && len(iface.Methods) == 0 && len(iface.TypeLists) == 0 {
				if u, ok := checkUsage(iface.EmbeddedIfaces[0]); ok {
					return u
				}
			}
		}
	}

	errs.Add(errTopicRefInvalidPerms.AtGoNode(expr.Call))
	return nil
}
