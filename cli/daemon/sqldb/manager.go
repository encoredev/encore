package sqldb

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

// NewClusterManager creates a new ClusterManager.
func NewClusterManager(driver Driver) *ClusterManager {
	log := log.Logger
	return &ClusterManager{
		log:            log,
		driver:         driver,
		clusters:       make(map[string]*Cluster),
		backendKeyData: make(map[uint32]*Cluster),
	}
}

// A ClusterManager manages running local sqldb clusters.
type ClusterManager struct {
	log        zerolog.Logger
	driver     Driver
	startGroup singleflight.Group

	mu       sync.Mutex
	clusters map[string]*Cluster // cluster id -> cluster
	// backendKeyData maps the secret data to a cluster,
	// for forwarding cancel requests to the right cluster.
	// Access is guarded by mu.
	backendKeyData map[uint32]*Cluster
}

// Create creates a database cluster but does not start it.
// If the cluster already exists it is returned.
// It does not perform any database migrations.
func (cm *ClusterManager) Create(ctx context.Context, params *CreateParams) *Cluster {
	cid := params.ClusterID
	cm.mu.Lock()
	defer cm.mu.Unlock()
	c, ok := cm.clusters[cid]
	if ok {
		if status, err := c.Status(ctx); err != nil || status.Status != Running {
			// The cluster is no longer running; recreate it to clear our cached state.
			c.cancel()
			ok = false
		}
	}
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		c = &Cluster{
			ID:      params.ClusterID,
			Memfs:   params.Memfs,
			Ctx:     ctx,
			driver:  cm.driver,
			cancel:  cancel,
			started: make(chan struct{}),
			log:     cm.log.With().Str("cluster", params.ClusterID).Logger(),
			dbs:     make(map[string]*DB),
		}
		cm.clusters[cid] = c
	}
	return c
}

// Get retrieves the cluster keyed by id.
func (cm *ClusterManager) Get(clusterID string) (*Cluster, bool) {
	cm.mu.Lock()
	c, ok := cm.clusters[clusterID]
	cm.mu.Unlock()
	return c, ok
}
