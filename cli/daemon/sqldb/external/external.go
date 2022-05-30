// Package external implements a cluster driver for an external cluster.
package external

import (
	"context"

	"github.com/rs/zerolog"

	"encr.dev/cli/daemon/sqldb"
)

type Driver struct {
	Host              string // "host", "host:port", "/path/to/unix.socket",
	Database          string // database name
	SuperuserUsername string
	SuperuserPassword string
}

var _ sqldb.Driver = (*Driver)(nil)

func (d *Driver) CreateCluster(ctx context.Context, p *sqldb.CreateParams, log zerolog.Logger) (*sqldb.ClusterStatus, error) {
	// The external driver does not actually create the cluster; just return the status.
	return d.ClusterStatus(ctx, p.ClusterID)
}

func (d *Driver) ClusterStatus(ctx context.Context, id sqldb.ClusterID) (*sqldb.ClusterStatus, error) {
	st := &sqldb.ClusterStatus{
		Status: sqldb.Running,
		Config: &sqldb.ConnConfig{
			Host: d.Host,
			Superuser: sqldb.Role{
				Type:     sqldb.RoleSuperuser,
				Username: def(d.SuperuserUsername, "postgres"),
				Password: def(d.SuperuserPassword, "postgres"),
			},
			RootDatabase: def(d.Database, "postgres"),
		},
	}
	return st, nil
}

func (d *Driver) DestroyCluster(ctx context.Context, id sqldb.ClusterID) error {
	return sqldb.ErrUnsupported
}

func def(val, orDefault string) string {
	if val == "" {
		val = orDefault
	}
	return val
}
