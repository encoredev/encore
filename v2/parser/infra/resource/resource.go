package resource

import (
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
)

type Parser struct {
	Name      string
	DependsOn []*Parser

	RequiredImports []string

	Run func(*Pass) []Resource
}

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
	Config
	Secrets
)

type Resource interface {
	Kind() Kind
	DeclaredIn() *pkginfo.File
}
