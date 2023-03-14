package api

import (
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/parser/resource/usage"
)

type CallUsage struct {
	usage.Base
}

func ResolveEndpointUsage(errs *perr.List, expr usage.Expr, endpoint *Endpoint) usage.Usage {
	switch expr := expr.(type) {
	case *usage.MethodCall:
		return &CallUsage{
			Base: usage.Base{
				File: expr.File,
				Bind: expr.Bind,
				Expr: expr,
			},
		}

	case *usage.FuncCall:
		return &CallUsage{
			Base: usage.Base{
				File: expr.File,
				Bind: expr.Bind,
				Expr: expr,
			},
		}
	}

	errs.Add(errInvalidEndpointUsage.AtGoNode(expr))

	return nil
}
