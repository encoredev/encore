package sqldb

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"

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
	App  *apps.Instance
	Type ClusterType

	NSID   namespace.ID
	NSName namespace.Name
}

func GetClusterID(app *apps.Instance, typ ClusterType, ns *namespace.Namespace) ClusterID {
	return ClusterID{app, typ, ns.ID, ns.Name}
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
	c, ok := cm.Get(GetClusterID(app, Run, ns))
	if !ok {
		return nil
	}

	err := c.driver.DestroyCluster(ctx, c.ID)
	if errors.Is(err, ErrUnsupported) {
		err = nil
	}
	return nil
}

type clusterKey string

// clusterKeys computes clusterKey candidates for a given id.
func clusterKeys(id ClusterID) []clusterKey {
	typeSuffix := "-" + string(id.Type)
	nsSuffix := "-" + string(id.NSID)

	var keys []clusterKey

	if pid := id.App.PlatformID(); pid != "" {
		keys = append(keys, clusterKey(pid+typeSuffix+nsSuffix))
	}
	keys = append(keys, clusterKey(id.App.LocalID()+typeSuffix+nsSuffix))

	return keys
}

func genPassword() string {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		log.Fatal().Err(err).Msg("unable to generate random data")
	}
	return base64.RawURLEncoding.EncodeToString(data[:])
}
