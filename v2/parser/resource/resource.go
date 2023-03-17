package resource

import (
	"go/ast"

	"encr.dev/v2/internal/pkginfo"
)

//go:generate stringer -type=Kind -output=resource_string.go

type Kind int

const (
	Unknown Kind = iota

	// Infrastructure SDK Resources
	PubSubTopic
	PubSubSubscription
	SQLDatabase
	Metric
	CronJob
	CacheCluster
	CacheKeyspace
	ConfigLoad
	Secrets

	// API Framework Resources
	APIEndpoint
	AuthHandler
	Middleware
	ServiceStruct
)

type Resource interface {
	// Node is embedded so we can use the resource in a [posmap.Map].
	// The position should be the position of the resource declaration.
	ast.Node

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
