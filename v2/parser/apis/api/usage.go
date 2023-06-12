package api

import (
	"go/ast"

	"encr.dev/v2/parser/resource/usage"
)

type CallUsage struct {
	usage.Base

	// Endpoint is the endpoint being called.
	Endpoint *HTTPEndpoint

	// Call is the function call expression.
	Call *ast.CallExpr
}

type ReferenceUsage struct {
	usage.Base

	// Endpoint is the endpoint being referenced.
	Endpoint *HTTPEndpoint

	// Ref is the reference expression.
	Ref ast.Expr
}

func ResolveEndpointUsage(data usage.ResolveData, ep *HTTPEndpoint) usage.Usage {
	switch expr := data.Expr.(type) {
	case *usage.FuncCall:
		return &CallUsage{
			Base: usage.Base{
				File: expr.File,
				Bind: expr.Bind,
				Expr: expr,
			},
			Endpoint: ep,
			Call:     expr.Call,
		}
	case *usage.Other:
		return &ReferenceUsage{
			Base: usage.Base{
				File: expr.File,
				Bind: expr.Bind,
				Expr: expr,
			},
			Endpoint: ep,
			Ref:      expr.BindRef,
		}
	}

	// Check if the resource is referenced in a permissible location.
	// Walk the stack to find the

	data.Errs.Add(ErrInvalidEndpointUsage.AtGoNode(data.Expr))
	return nil
}
