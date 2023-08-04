package sqldb

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
)

// NewClusterManager creates a new ClusterManager.
func NewClusterManager(driver Driver, apps *apps.Manager, ns *namespace.Manager) *ClusterManager {
	log := log.Logger
	return &ClusterManager{
		log:            log,
		driver:         driver,
		apps:           apps,
		ns:             ns,
		clusters:       make(map[clusterKey]*Cluster),
		backendKeyData: make(map[uint32]*Cluster),
	}
}

// A ClusterManager manages running local sqldb clusters.
type ClusterManager struct {
	log        zerolog.Logger
	driver     Driver
	apps       *apps.Manager
	ns         *namespace.Manager
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
	NS   *namespace.Namespace
	Type ClusterType
}

// clusterKey is the key to use to store a cluster in the cluster map.
type clusterKey string

func (id ClusterID) clusterKey() clusterKey {
	return clusterKey(fmt.Sprintf("%s-%s", id.NS.ID, id.Type))
}

func GetClusterID(app *apps.Instance, typ ClusterType, ns *namespace.Namespace) ClusterID {
	return ClusterID{ns, typ}
}

// Ready reports whether the cluster manager is ready and all requirements are met.
func (cm *ClusterManager) Ready() error {
	return cm.driver.CheckRequirements(context.Background())
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
		key := params.ClusterID.clusterKey()
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
	c, ok := cm.clusters[id.clusterKey()]
	return c, ok
}

// CanDeleteNamespace implements namespace.DeletionHandler.
func (cm *ClusterManager) CanDeleteNamespace(ctx context.Context, app *apps.Instance, ns *namespace.Namespace) error {
	c, ok := cm.Get(GetClusterID(app, Run, ns))
	if !ok {
		return nil
	}

	err := c.driver.CanDestroyCluster(ctx, c.ID)
	if errors.Is(err, ErrUnsupported) {
		err = nil
	}
	return nil
}

// DeleteNamespace implements namespace.DeletionHandler.
func (cm *ClusterManager) DeleteNamespace(ctx context.Context, app *apps.Instance, ns *namespace.Namespace) error {
	// Find all clusters matching this namespace.
	// Use a closure for the lock to avoid holding it while we destroy the clusters.
	var clusters []*Cluster
	(func() {
		cm.mu.Lock()
		defer cm.mu.Unlock()
		for _, c := range cm.clusters {
			if c.ID.NS.ID == ns.ID {
				clusters = append(clusters, c)
			}
		}
	})()

	// Destroy the clusters.
	for _, c := range clusters {
		if err := c.driver.DestroyCluster(ctx, c.ID); err != nil && !errors.Is(err, ErrUnsupported) {
			return errors.Wrapf(err, "destroy cluster %s", c.ID)
		}
		c.cancel()
	}

	// If that succeeded, destroy the namespace data.
	err := cm.driver.DestroyNamespaceData(ctx, ns)
	if errors.Is(err, ErrUnsupported) {
		err = nil
	}
	return err
}

func genPassword() string {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		log.Fatal().Err(err).Msg("unable to generate random data")
	}
	return base64.RawURLEncoding.EncodeToString(data[:])
}
