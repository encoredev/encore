package sqldb

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"encr.dev/cli/daemon/apps"
)

// NewClusterManager creates a new ClusterManager.
func NewClusterManager(driver Driver) *ClusterManager {
	log := log.Logger
	return &ClusterManager{
		log:            log,
		driver:         driver,
		clusters:       make(map[clusterKey]*Cluster),
		backendKeyData: make(map[uint32]*Cluster),
	}
}

// A ClusterManager manages running local sqldb clusters.
type ClusterManager struct {
	log        zerolog.Logger
	driver     Driver
	startGroup singleflight.Group

	mu       sync.Mutex
	clusters map[clusterKey]*Cluster
	// backendKeyData maps the secret data to a cluster,
	// for forwarding cancel requests to the right cluster.
	// Access is guarded by mu.
	backendKeyData map[uint32]*Cluster
}

// ClusterID uniquely identifies a cluster.
type ClusterID struct {
	App  *apps.Instance
	Type ClusterType
}

func GetClusterID(app *apps.Instance, typ ClusterType) ClusterID {
	return ClusterID{app, typ}
}

// Create creates a database cluster but does not start it.
// If the cluster already exists it is returned.
// It does not perform any database migrations.
func (cm *ClusterManager) Create(ctx context.Context, params *CreateParams) *Cluster {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	c, ok := cm.get(params.ClusterID)
	if ok {
		if status, err := c.Status(ctx); err != nil || status.Status != Running {
			// The cluster is no longer running; recreate it to clear our cached state.
			c.cancel()
			ok = false
		}
	}

	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		key := clusterKeys(params.ClusterID)[0] // guaranteed to be non-empty
		passwd := genPassword()
		c = &Cluster{
			ID:       params.ClusterID,
			Memfs:    params.Memfs,
			Password: passwd,
			Ctx:      ctx,
			driver:   cm.driver,
			cancel:   cancel,
			started:  make(chan struct{}),
			log:      cm.log.With().Interface("cluster", params.ClusterID).Logger(),
			dbs:      make(map[string]*DB),
		}

		cm.clusters[key] = c
	}

	return c
}

// LookupPassword looks up a cluster based on its password.
func (cm *ClusterManager) LookupPassword(password string) (*Cluster, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, c := range cm.clusters {
		if c.Password == password {
			return c, true
		}
	}
	return nil, false
}

// Get retrieves the cluster keyed by id.
func (cm *ClusterManager) Get(id ClusterID) (*Cluster, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.get(id)
}

// get retrieves the cluster keyed by id.
// cm.mu must be held.
func (cm *ClusterManager) get(id ClusterID) (*Cluster, bool) {
	for _, key := range clusterKeys(id) {
		if c, ok := cm.clusters[key]; ok {
			return c, true
		}
	}
	return nil, false
}

type clusterKey string

// clusterKeys computes clusterKey candidates for a given id.
func clusterKeys(id ClusterID) []clusterKey {
	suffix := "-" + string(id.Type)
	var keys []clusterKey
	if pid := id.App.PlatformID(); pid != "" {
		keys = append(keys, clusterKey(pid+suffix))
	}
	keys = append(keys, clusterKey(id.App.LocalID()+suffix))
	return keys
}

func genPassword() string {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		log.Fatal().Err(err).Msg("unable to generate random data")
	}
	return base64.RawURLEncoding.EncodeToString(data[:])
}
