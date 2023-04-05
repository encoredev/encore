//go:build encore_app

package cache

import (
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/jsonapi"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/testsupport"
)

//publicapigen:drop
var Singleton = NewManager(appconf.Static, appconf.Runtime, reqtrack.Singleton, testsupport.Singleton, jsonapi.Default)

// NewCluster declares a new cache cluster.
//
// See https://encore.dev/docs/develop/caching for more information.
func NewCluster(name string, cfg ClusterConfig) *Cluster {
	return &Cluster{
		cfg: cfg,
		mgr: Singleton,
		cl:  Singleton.getClient(name),
	}
}
