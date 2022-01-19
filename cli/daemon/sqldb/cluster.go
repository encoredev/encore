package sqldb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go4.org/syncutil"
	"golang.org/x/sync/errgroup"

	"encr.dev/cli/daemon/internal/runlog"
	meta "encr.dev/proto/encore/parser/meta/v1"

	// stdlib registers the "pgx" driver to database/sql.
	_ "github.com/jackc/pgx/v4/stdlib"
)

// Cluster represents a running database Cluster.
type Cluster struct {
	ID    string // cluster ID
	Memfs bool   // use an an in-memory filesystem?

	HostPort string // available after Ready() is done

	log zerolog.Logger

	startOnce syncutil.Once
	// started is closed when the cluster has been successfully started.
	started chan struct{}

	// Ctx is canceled when the cluster is being torn down.
	Ctx    context.Context
	cancel func() // for canceling Ctx

	mu  sync.Mutex
	dbs map[string]*DB // name -> db
}

// Ready returns a channel that is closed when the cluster is up and running.
func (c *Cluster) Ready() <-chan struct{} {
	return c.started
}

// Start creates the container if necessary and starts it.
// If the cluster is already running it does nothing.
func (c *Cluster) Start(log runlog.Log) error {
	return c.startOnce.Do(func() (err error) {
		c.log.Debug().Msg("starting cluster")
		defer func() {
			if err == nil {
				close(c.started)
				c.log.Debug().Msg("successfully started cluster")
			} else {
				c.log.Error().Err(err).Msg("failed to start cluster")
			}
		}()

		// Ensure the docker image exists first.
		if err := PullImage(context.Background()); err != nil {
			return fmt.Errorf("pull docker image: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		cname := containerName(c.ID)
		status, err := c.Status(ctx)
		if err != nil {
			c.log.Error().Err(err).Msg("failed to get container status")
			return err
		}

		// waitForPort waits for the port to become available before assigning it to c.HostPort.
		waitForPort := func() error {
			for i := 0; i < 20; i++ {
				status, err = c.Status(ctx)
				if err != nil {
					return err
				}
				if status.HostPort != "" {
					c.HostPort = status.HostPort
					c.log.Debug().Str("hostport", c.HostPort).Msg("cluster started")
					return nil
				}
				time.Sleep(500 * time.Millisecond)
			}
			return fmt.Errorf("timed out waiting for cluster to start")
		}

		switch status.Status {
		case Running:
			c.HostPort = status.HostPort
			c.log.Debug().Str("hostport", c.HostPort).Msg("cluster already running")
			return nil

		case Stopped:
			c.log.Debug().Msg("cluster stopped, restarting")
			if out, err := exec.CommandContext(ctx, "docker", "start", cname).CombinedOutput(); err != nil {
				return fmt.Errorf("could not start sqldb container: %s (%v)", string(out), err)
			}
			return waitForPort()

		case NotFound:
			c.log.Debug().Msg("cluster not found, creating")
			args := []string{
				"run",
				"-d",
				"-p", "5432",
				"--shm-size=1gb",
				"-e", "POSTGRES_USER=encore",
				"-e", "POSTGRES_PASSWORD=" + c.ID,
				"-e", "POSTGRES_DB=postgres",
				"--name", cname,
			}
			if c.Memfs {
				args = append(args,
					"--mount", "type=tmpfs,destination=/var/lib/postgresql/data",
					dockerImage,
					"-c", "fsync=off",
				)
			} else {
				args = append(args, dockerImage)
			}

			cmd := exec.CommandContext(ctx, "docker", args...)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("could not start sql database as docker container: %s: %v", out, err)
			}

			c.log.Debug().Str("hostport", c.HostPort).Msg("cluster created")
			return waitForPort()

		default:
			return fmt.Errorf("unknown cluster status %q", status.Status)
		}
	})
}

// initDBs adds the databases from md to the cluster's database map.
// It does not create or migrate them.
func (c *Cluster) initDBs(md *meta.Data, reinit bool) {
	if md == nil {
		return
	}

	// Create the databases we need in our cluster map.
	c.mu.Lock()
	for _, svc := range md.Svcs {
		if len(svc.Migrations) > 0 {
			db, ok := c.dbs[svc.Name]
			if ok && reinit {
				db.CloseConns()
			}
			if !ok || reinit {
				c.initDB(svc.Name)
			}
		}
	}
	c.mu.Unlock()
}

// initDB initializes the database for svc and adds it to c.dbs.
// The cluster mutex must be held.
func (c *Cluster) initDB(name string) *DB {
	dbCtx, cancel := context.WithCancel(c.Ctx)
	db := &DB{
		Name:    name,
		Cluster: c,

		Ctx:    dbCtx,
		cancel: cancel,

		ready: make(chan struct{}),
		log:   c.log.With().Str("db", name).Logger(),
	}
	c.dbs[name] = db
	return db
}

// Create creates the given databases.
func (c *Cluster) Create(ctx context.Context, appRoot string, md *meta.Data) error {
	c.log.Debug().Msg("creating cluster")
	g, ctx := errgroup.WithContext(ctx)
	c.mu.Lock()
	for _, svc := range md.Svcs {
		if len(svc.Migrations) == 0 {
			continue
		}

		svc := svc
		db, ok := c.dbs[svc.Name]
		if !ok {
			c.mu.Unlock()
			return fmt.Errorf("database %s not initialized", svc.Name)
		}
		g.Go(func() error { return db.Setup(ctx, appRoot, svc, false, false) })
	}
	c.mu.Unlock()
	return g.Wait()
}

// CreateAndMigrate creates and migrates the given databases.
func (c *Cluster) CreateAndMigrate(ctx context.Context, appRoot string, md *meta.Data) error {
	c.log.Debug().Msg("creating and migrating cluster")
	g, ctx := errgroup.WithContext(ctx)
	c.mu.Lock()
	for _, svc := range md.Svcs {
		if len(svc.Migrations) == 0 {
			continue
		}

		svc := svc
		db, ok := c.dbs[svc.Name]
		if !ok {
			c.mu.Unlock()
			return fmt.Errorf("database %s not initialized", svc.Name)
		}
		g.Go(func() error { return db.Setup(ctx, appRoot, svc, true, false) })
	}
	c.mu.Unlock()
	return g.Wait()
}

// GetDB gets the database with the given name.
func (c *Cluster) GetDB(name string) (*DB, bool) {
	c.mu.Lock()
	db, ok := c.dbs[name]
	c.mu.Unlock()
	return db, ok
}

// Recreate recreates the databases for the given services.
// If services is the nil slice it recreates all databases.
func (c *Cluster) Recreate(ctx context.Context, appRoot string, services []string, md *meta.Data) error {
	c.log.Debug().Msg("recreating cluster")
	var filter map[string]bool
	if services != nil {
		filter = make(map[string]bool)
		for _, svc := range services {
			filter[svc] = true
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	c.mu.Lock()
	for _, svc := range md.Svcs {
		svc := svc
		if len(svc.Migrations) > 0 && (filter == nil || filter[svc.Name]) {
			db, ok := c.dbs[svc.Name]
			if !ok {
				db = c.initDB(svc.Name)
			}
			g.Go(func() error { return db.Setup(ctx, appRoot, svc, true, true) })
		}
	}
	c.mu.Unlock()
	err := g.Wait()
	c.log.Debug().Err(err).Msg("recreated cluster")
	return err
}

// Status reports the status of the cluster.
func (c *Cluster) Status(ctx context.Context) (*ClusterStatus, error) {
	cname := containerName(c.ID)
	out, err := exec.CommandContext(ctx, "docker", "container", "inspect", cname).CombinedOutput()
	if err == exec.ErrNotFound {
		return nil, errors.New("docker not found: is it installed and in your PATH?")
	} else if err != nil {
		// Docker returns a non-zero exit code if the container does not exist.
		// Try to tell this apart from an error by parsing the output.
		if bytes.Contains(out, []byte("No such container")) {
			return &ClusterStatus{Status: NotFound}, nil
		}
		return nil, fmt.Errorf("docker container inspect failed: %s (%v)", out, err)
	}

	var resp []struct {
		Name  string
		State struct {
			Running bool
		}
		NetworkSettings struct {
			Ports map[string][]struct {
				HostIP   string
				HostPort string
			}
		}
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parse `docker container inspect` response: %v", err)
	}
	for _, c := range resp {
		if c.Name == "/"+cname {
			status := &ClusterStatus{Status: Stopped}
			if c.State.Running {
				status.Status = Running
			}
			ports := c.NetworkSettings.Ports["5432/tcp"]
			if len(ports) > 0 {
				status.HostPort = ports[0].HostIP + ":" + ports[0].HostPort
			}
			return status, nil
		}
	}
	return &ClusterStatus{Status: NotFound}, nil
}

// ContainerStatus represents the status of a container.
type ContainerStatus string

const (
	// Running indicates the cluster container is running.
	Running ContainerStatus = "running"
	// Stopped indicates the container cluster exists but is not running.
	Stopped ContainerStatus = "stopped"
	// NotFound indicates the container cluster does not exist.
	NotFound ContainerStatus = "notfound"
)

// ClusterStatus rerepsents the status of a database cluster.
type ClusterStatus struct {
	// Status is the status of the underlying container.
	Status ContainerStatus
	// HostPort is the host and port for connecting to the database.
	// It is only set when Status == Running.
	HostPort string
}

// containerName computes the container name for a given clusterID.
func containerName(clusterID string) string {
	return "sqldb-" + clusterID
}

// ImageExists reports whether the docker image exists.
func ImageExists(ctx context.Context) (ok bool, err error) {
	out, err := exec.CommandContext(ctx, "docker", "image", "inspect", dockerImage).CombinedOutput()
	switch {
	case err == nil:
		return true, nil
	case bytes.Contains(out, []byte("No such image")):
		return false, nil
	default:
		return false, err
	}
}

// PullImage pulls the image.
func PullImage(ctx context.Context) error {
	if ok, _ := ImageExists(ctx); ok {
		return nil
	}
	cmd := exec.CommandContext(ctx, "docker", "pull", dockerImage)
	return cmd.Run()
}
