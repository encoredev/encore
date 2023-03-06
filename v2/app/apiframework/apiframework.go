package apiframework

import (
	"encr.dev/pkg/option"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/servicestruct"
	"encr.dev/v2/parser/infra"
)

// AppDesc describes an Encore Framework-based application.
type AppDesc struct {
	errs *perr.List

	// GlobalMiddleware is the list of application-global middleware.
	GlobalMiddleware []*middleware.Middleware

	// AuthHandler defines the application's auth handler, if any.
	AuthHandler option.Option[*authhandler.AuthHandler]
}

// ServiceDesc describes an Encore Framework-based service.
//
// For code that deals with general services, use *service.Service instead of this type.
type ServiceDesc struct {
	// Num is the service number in the application.
	Num int

	// Middleware are the service-specific middleware
	Middleware []*middleware.Middleware

	// RootPkg is the root package of the service.
	RootPkg *pkginfo.Package

	// Endpoints are the endpoints defined in this service.
	Endpoints []*api.Endpoint

	// ServiceStruct defines the service's service struct, if any.
	ServiceStruct option.Option[*servicestruct.ServiceStruct]
}

// NewBuilder creates a new Builder.
func NewBuilder(pc *parsectx.Context) *Builder {
	return &Builder{pc: pc, errs: pc.Errs}
}

// Builder is used to build an application description.
type Builder struct {
	pc      *parsectx.Context
	errs    *perr.List
	results []*apis.ParseResult
	infra   *infra.ParseResult
}
