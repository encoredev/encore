package sqldb

import (
	"context"
	"errors"

	"github.com/rs/zerolog"

	"encr.dev/internal/optracker"
)

var ErrUnsupported = errors.New("unsupported operation")

// A Driver abstracts away how a cluster is actually operated.
type Driver interface {
	// CreateCluster creates (if necessary) and starts (if necessary) a new cluster using the driver,
	// and returns its status.
	// err is nil if and only if the cluster could not be started.
	CreateCluster(ctx context.Context, p *CreateParams, log zerolog.Logger) (*ClusterStatus, error)

	// DestroyCluster destroys a cluster with the given id.
	// If a Driver doesn't support destroying the cluster it reports ErrUnsupported.
	DestroyCluster(ctx context.Context, id ClusterID) error

	// ClusterStatus reports the current status of a cluster.
	ClusterStatus(ctx context.Context, id ClusterID) (*ClusterStatus, error)

	// CheckRequirements checks whether all the requirements are met
	// to use the driver.
	CheckRequirements(ctx context.Context) error

	// Meta reports driver metadata.
	Meta() DriverMeta
}

type DriverMeta struct {
	// ClusterIsolation reports whether clusters are isolated by the driver.
	// If false, database names will be prefixed with the cluster id.
	ClusterIsolation bool
}

type ConnConfig struct {
	// Host is the host address to connect to the database.
	// It is only set when Status == Running.
	Host string

	// Superuser is the role to use to connect as the superuser,
	// for creating and managing Encore databases.
	Superuser    Role
	RootDatabase string // root database to connect to
}

type ClusterType string

const (
	Run  ClusterType = "run"
	Test ClusterType = "test"
)

// CreateParams are the params to (*ClusterManager).Create.
type CreateParams struct {
	ClusterID ClusterID

	// Memfs, if true, configures the database container to use an
	// in-memory filesystem as opposed to persisting the database to disk.
	Memfs bool

	// Tracker allows tracking the progress of the operation.
	Tracker *optracker.OpTracker
}

// Status represents the status of a container.
type Status string

const (
	// Running indicates the cluster is running.
	Running Status = "running"
	// Stopped indicates the container exists but is not running.
	Stopped Status = "stopped"
	// NotFound indicates the container does not exist.
	NotFound Status = "notfound"
)

// ClusterStatus represents the status of a database cluster.
type ClusterStatus struct {
	// Status is the status of the underlying container.
	Status Status

	// Config is how to connect to the cluster.
	// It is non-nil if Status == Running.
	Config *ConnConfig
}
