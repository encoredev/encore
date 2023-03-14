package api

import (
	"go/ast"

	"encr.dev/v2/parser/resource/usage"
)

type CallUsage struct {
	usage.Base

	// Endpoint is the endpoint being called.
	Endpoint *Endpoint

	// Call is the function call expression.
	Call *ast.CallExpr
}

func ResolveEndpointUsage(data usage.ResolveData, ep *Endpoint) usage.Usage {
	if fc, ok := data.Expr.(*usage.FuncCall); ok {
		return &CallUsage{
			Base: usage.Base{
				File: fc.File,
				Bind: fc.Bind,
				Expr: fc,
			},
			Endpoint: ep,
			Call:     fc.Call,
		}
	}

	// Check if the resource is referenced in a permissible location.
	// Walk the stack to find the

	data.Errs.Add(errInvalidEndpointUsage.AtGoNode(data.Expr))
	return nil
}
