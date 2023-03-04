package apiframework

import (
	"encr.dev/pkg/option"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/servicestruct"
	"encr.dev/v2/parser/service"
)

// AppDesc describes an Encore Framework-based application.
type AppDesc struct {
	Services   []*Service
	Middleware []*middleware.Middleware

	// AuthHandler defines the application's auth handler, if any.
	AuthHandler option.Option[*authhandler.AuthHandler]
}

// Service is an Encore Framework-based service.
//
// For code that deals with general services, use *service.Service instead of this type.
type Service struct {
	*service.Service

	// Num is the service number in the application.
	Num int

	// RootPkg is the root package of the service.
	RootPkg *pkginfo.Package

	// Endpoints are the endpoints defined in this service.
	Endpoints []*api.Endpoint

	// ServiceStruct defines the service's service struct, if any.
	ServiceStruct option.Option[*servicestruct.ServiceStruct]
}

// NewBuilder creates a new Builder.
func NewBuilder(errs *perr.List) *Builder {
	return &Builder{errs: errs}
}

// Builder is used to build an application description.
type Builder struct {
	errs    *perr.List
	results []*apis.ParseResult
}

// AddResult adds a parse result for a specific package.
func (b *Builder) AddResult(res *apis.ParseResult) {
	b.results = append(b.results, res)
}

// Build computes the application description.
func (b *Builder) Build() *AppDesc {
	// TODO
	return nil
}
