package config

import (
	"encr.dev/v2/parser/resource/usage"
)

type ReferenceUsage struct {
	usage.Base

	// Load is the config load being used.
	Cfg *Load
}

func ResolveConfigUsage(data usage.ResolveData, load *Load) usage.Usage {
	return &ReferenceUsage{
		Base: usage.Base{
			File: data.Expr.DeclaredIn(),
			Bind: data.Expr.ResourceBind(),
			Expr: data.Expr,
		},
		Cfg: load,
	}
}
