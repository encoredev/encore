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

type ReferenceUsage struct {
	usage.Base

	// Endpoint is the endpoint being referenced.
	Endpoint *Endpoint

	// Ref is the reference expression.
	Ref ast.Expr
}

// ClientCallUsage tracks calls to generated client methods
type ClientCallUsage struct {
	usage.Base

	// Endpoint is the endpoint being called through the client.
	Endpoint *Endpoint

	// ServiceName is the name of the target service.
	ServiceName string

	// EndpointName is the name of the endpoint method.
	EndpointName string

	// ClientCall is the client method call expression.
	ClientCall *ast.CallExpr
}

func (c *ClientCallUsage) usage() {}

// ClientReferenceUsage tracks references to client constructors
type ClientReferenceUsage struct {
	usage.Base

	// ServiceName is the name of the service being referenced.
	ServiceName string

	// ClientRef is the client reference expression.
	ClientRef ast.Expr
}

func (c *ClientReferenceUsage) usage() {}

func ResolveEndpointUsage(data usage.ResolveData, ep *Endpoint) usage.Usage {
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
	case *usage.FuncArg:
		// If this is a test file and we're calling `et.MockEndpoint[WithMiddleware]` this is allowed.
		if pkg, ok := expr.PkgFunc.Get(); ok && expr.DeclaredIn().TestFile && pkg.PkgPath == "encore.dev/et" && pkg.Name == "MockEndpoint" {
			return &ReferenceUsage{
				Base: usage.Base{
					File: expr.File,
					Bind: expr.Bind,
					Expr: expr,
				},
				Endpoint: ep,
				Ref:      expr.ASTExpr(),
			}
		}
	}

	// Check if the resource is referenced in a permissible location.
	// Walk the stack to find the

	data.Errs.Add(ErrInvalidEndpointUsage.AtGoNode(data.Expr))
	return nil
}
