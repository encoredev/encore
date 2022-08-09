//go:build encore_app

package cache

var Singleton *Manager

func NewCluster(name string, cfg ClusterConfig) *Cluster {
	return &Cluster{
		cl: Singleton.redis,
	}
}
