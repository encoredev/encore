package resource

import (
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
)

type Parser struct {
	Name            string
	RequiredImports []paths.Pkg
	Run             func(*Pass) []Resource
}

// RunAlways is a value for RequiredImports to indicate to always run the parser.
var RunAlways = []paths.Pkg{"*"}

type Pass struct {
	*parsectx.Context
	SchemaParser *schema.Parser

	Pkg *pkginfo.Package
}

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
	Kind() Kind
	DeclaredIn() *pkginfo.File
}

type Reference struct {
	Kind Kind
	Name string
}
