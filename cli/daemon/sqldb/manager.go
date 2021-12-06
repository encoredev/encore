// Package sqldb runs and manages connections for Encore applications.
package sqldb

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"

	meta "encr.dev/proto/encore/parser/meta/v1"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

// NewClusterManager creates a new ClusterManager.
func NewClusterManager() *ClusterManager {
	log := log.Logger
	return &ClusterManager{
		log:            log,
		clusters:       make(map[string]*Cluster),
		backendKeyData: make(map[uint32]*Cluster),
	}
}

// A ClusterManager manages running local sqldb clusters.
type ClusterManager struct {
	log        zerolog.Logger
	startGroup singleflight.Group

	mu       sync.Mutex
	clusters map[string]*Cluster // cluster id -> cluster
	// backendKeyData maps the secret data to a cluster,
	// for forwarding cancel requests to the right cluster.
	// Access is guarded by mu.
	backendKeyData map[uint32]*Cluster
}

// InitParams are the params to (*ClusterManager).Init.
type InitParams struct {
	// ClusterID is the unique id of the cluster.
	ClusterID string

	// Meta is the metadata used to initialize databases.
	// If nil no databases are initialized.
	Meta *meta.Data

	// Memfs, if true, configures the database container to use an
	// in-memory filesystem as opposed to persisting the database to disk.
	Memfs bool

	// Reinit forces all databases to be reinitialized, even if they already exist.
	Reinit bool
}

// Init initializes a database cluster but does not start it.
// If the cluster already exists it is returned.
// It does not perform any database migrations.
func (cm *ClusterManager) Init(ctx context.Context, params *InitParams) *Cluster {
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
			cancel:  cancel,
			started: make(chan struct{}),
			log:     cm.log.With().Str("cluster", params.ClusterID).Logger(),
			dbs:     make(map[string]*DB),
		}
		cm.clusters[cid] = c
	}
	c.initDBs(params.Meta, params.Reinit)
	return c
}

// Get retrieves the cluster keyed by id.
func (cm *ClusterManager) Get(clusterID string) (*Cluster, bool) {
	cm.mu.Lock()
	c, ok := cm.clusters[clusterID]
	cm.mu.Unlock()
	return c, ok
}

// Delete forcibly deletes the cluster.
func (cm *ClusterManager) Delete(ctx context.Context, clusterID string) error {
	cname := containerName(clusterID)
	out, err := exec.CommandContext(ctx, "docker", "rm", "-f", cname).CombinedOutput()
	if err != nil {
		if bytes.Contains(out, []byte("No such container")) {
			return nil
		}
		return fmt.Errorf("could not delete cluster: %s (%v)", out, err)
	}
	return nil
}

const dockerImage = "postgres:11-alpine"
