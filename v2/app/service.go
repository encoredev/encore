package app

import (
	"encr.dev/pkg/option"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/usage"
)

// Service describes an Encore service.
type Service struct {
	// Name is the name of the service.
	Name string

	// FSRoot is the root directory of the service.
	FSRoot paths.FS

	// Framework contains API Framework-specific data for this service.
	Framework option.Option[*apiframework.ServiceDesc]

	// ResourceUsage describes the infra resources the service accesses and how.
	ResourceUsage map[resource.Resource][]usage.Usage
}

// ContainsPackage reports whether the service contains the given package.
func (s *Service) ContainsPackage(pkg *pkginfo.Package) bool {
	return s.FSRoot.HasPrefix(pkg.FSPath)
}
