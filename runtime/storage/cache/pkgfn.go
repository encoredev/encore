//go:build encore_app

package cache

var Singleton *Manager

// NewCluster declares a new cache cluster.
func NewCluster(name string, cfg ClusterConfig) *Cluster {
	return &Cluster{
		cfg: cfg,
		mgr: Singleton,
		cl:  Singleton.getClient(name),
	}
}
