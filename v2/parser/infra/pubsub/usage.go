package pubsub

import (
	"encr.dev/pkg/option"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
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

	if schemautil.IsNamed(expr.TypeArgs[0], "encore.dev/pubsub", "Publisher") {
		return &RefUsage{
			Base: usage.Base{
				File: expr.File,
				Bind: expr.Bind,
				Expr: expr,
			},
			Perms: []Perm{PublishPerm},
		}
	} else {
		errs.Add(errTopicRefInvalidPerms.AtGoNode(expr.Call))
		return nil
	}
}
