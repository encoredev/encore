package servicestruct

import (
	"encr.dev/v2/parser/resource/usage"
)

type Usage struct {
	usage.Base

	ServiceStruct *ServiceStruct
}

func ResolveServiceStructUsage(data usage.ResolveData, s *ServiceStruct) usage.Usage {
	return &Usage{
		Base: usage.Base{
			File: data.Expr.DeclaredIn(),
			Bind: data.Expr.ResourceBind(),
			Expr: data.Expr,
		},
		ServiceStruct: s,
	}
}
