//go:build encore_app

package cache

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
