package resource

import (
	"go/ast"
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
	TracedFunc
)

type Resource interface {
	// Node is embedded so we can use the resource in a [posmap.Map].
	// The position should be the position of the resource declaration.
	ast.Node

	// Kind is the kind of resource this is.
	Kind() Kind

	// SortKey is a string that can be used to sort resources.
	// The sort key's value is arbitrary but should provide a
	// consistent sort order regardless of the parsing order.
	// The sort key only matters between resources of the same Kind.
	SortKey() string
}

type Named interface {
	Resource

	// ResourceName is the name of the resource.
	ResourceName() string
}
