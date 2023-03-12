package resource

import (
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
)

type Parser struct {
	Name string

	// InterestingImports are the imports paths that the parser is interested in.
	// If a package imports any one of them, the Run method is invoked.
	InterestingImports []paths.Pkg

	// InterestingSubdirs are the subdirectories of a package that a parser is interested in.
	// If a package has any one of these subdirectories, the Run method is invoked.
	// Its purpose is to support our current way of defining databases via a "migrations" dir.
	InterestingSubdirs []string

	Run func(*Pass)
}

// RunAlways is a value for InterestingImports to indicate to always run the parser.
var RunAlways = []paths.Pkg{"*"}

type Pass struct {
	*parsectx.Context
	SchemaParser *schema.Parser

	Pkg *pkginfo.Package

	resources []Resource
	binds     []Bind
}

func (p *Pass) RegisterResource(resource Resource) {
	p.resources = append(p.resources, resource)
}

func (p *Pass) Resources() []Resource {
	return p.resources
}

func (p *Pass) Binds() []Bind {
	return p.binds
}

//go:generate stringer -type=Kind -output=resource_string.go

type Kind int

const (
	Unknown Kind = iota
	PubSubTopic
	PubSubSubscription
	SQLDatabase
	Metric
	CronJob
	CacheCluster
	CacheKeyspace
	ConfigLoad
	Secrets
)

type Resource interface {
	// Kind is the kind of resource this is.
	Kind() Kind

	// Package is the package the resource is declared in.
	Package() *pkginfo.Package
}

type Named interface {
	Resource

	// ResourceName is the name of the resource.
	ResourceName() string
}
