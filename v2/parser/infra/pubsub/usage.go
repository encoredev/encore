package pubsub

import (
	"encr.dev/pkg/option"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/resource/usage"
)

type PublishUsage struct {
	usage.Base
}

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
		}

	case *usage.FuncArg:
		switch {
		case option.Contains(expr.PkgFunc, pkginfo.Q("encore.dev/pubsub", "NewSubscription")):
			// Allowed usage
			return nil
		case option.Contains(expr.PkgFunc, pkginfo.Q("encore.dev/et", "Topic")):
			// Allowed usage
			return nil
		}
	}

	data.Errs.Add(errInvalidTopicUsage.AtGoNode(data.Expr))
	return nil
}
