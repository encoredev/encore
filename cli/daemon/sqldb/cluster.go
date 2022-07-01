package sqldb

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v4"
	"github.com/rs/zerolog"
	"go4.org/syncutil"
	"golang.org/x/sync/errgroup"

	meta "encr.dev/proto/encore/parser/meta/v1"

	// stdlib registers the "pgx" driver to database/sql.
	_ "github.com/jackc/pgx/v4/stdlib"
)

// Cluster represents a running database Cluster.
type Cluster struct {
	ID       ClusterID // cluster ID
	Memfs    bool      // use an in-memory filesystem?
	Password string    // randomly generated password for this cluster

	driver Driver
	log    zerolog.Logger

	startOnce syncutil.Once
	// started is closed when the cluster has been successfully started.
	started chan struct{}

	Roles EncoreRoles // set by Start

	// Ctx is canceled when the cluster is being torn down.
	Ctx    context.Context
	cancel func() // for canceling Ctx

	mu  sync.Mutex
	dbs map[string]*DB // name -> db
}

func (c *Cluster) Stop() {
	// no-op
}

// Ready returns a channel that is closed when the cluster is up and running.
func (c *Cluster) Ready() <-chan struct{} {
	return c.started
}

// Start creates the cluster if necessary and starts it.
// If the cluster is already running it does nothing.
func (c *Cluster) Start(ctx context.Context) (*ClusterStatus, error) {
	var status *ClusterStatus
	err := c.startOnce.Do(func() (err error) {
		c.log.Debug().Msg("starting cluster")
		defer func() {
			if err == nil {
				close(c.started)
				c.log.Debug().Msg("successfully started cluster")
			} else {
				c.log.Error().Err(err).Msg("failed to start cluster")
			}
		}()

		st, err := c.driver.CreateCluster(ctx, &CreateParams{
			ClusterID: c.ID,
			Memfs:     c.Memfs,
		}, c.log)
		if err != nil {
			return err
		}
		status = st

		// Setup the roles
		c.Roles, err = c.setupRoles(ctx, st)

		return err
	})

	if err != nil {
		return nil, err
	} else if status == nil {
		// We've already set it up; query the current status
		return c.Status(ctx)
	}
	return status, nil
}

// setupRoles ensures the necessary database roles exist
// for admin/write/read access.
func (c *Cluster) setupRoles(ctx context.Context, st *ClusterStatus) (EncoreRoles, error) {
	uri := st.ConnURI(st.Config.RootDatabase, st.Config.Superuser)
	conn, err := pgx.Connect(ctx, uri)
	if err != nil {
		return nil, fmt.Errorf("connect: %v", err)
	}
	defer conn.Close(context.Background())

	roles, err := c.determineRoles(ctx, st, conn)
	if err != nil {
		return nil, fmt.Errorf("determine roles: %v", err)
	}

	for _, role := range roles {
		sanitizedUsername := (pgx.Identifier{role.Username}).Sanitize()
		c.log.Debug().Str("role", role.Username).Msg("creating role")
		_, err := conn.Exec(ctx, `
			CREATE USER `+sanitizedUsername+`
			WITH LOGIN ENCRYPTED PASSWORD `+quoteString(role.Password)+`
		`)
		if err != nil {
			var exists bool
			err2 := conn.QueryRow(context.Background(), `
				SELECT COALESCE(MAX(oid), 0) > 0 AS exists
				FROM pg_roles
				WHERE rolname = $1
			`, role.Username).Scan(&exists)
			if err2 != nil {
				c.log.Error().Err(err2).Str("role", role.Username).Msg("unable to lookup role")
				return nil, fmt.Errorf("get role %q: %v", role.Username, err2)
			} else if !exists {
				c.log.Error().Err(err).Str("role", role.Username).Msg("unable to create role")
				return nil, fmt.Errorf("create role %q: %v", role.Username, err)
			}
			c.log.Debug().Str("role", role.Username).Msg("role already exists")
		}
	}

	return roles, nil
}

// determineRoles determines the roles to create based on the server version.
func (c *Cluster) determineRoles(ctx context.Context, st *ClusterStatus, conn *pgx.Conn) (EncoreRoles, error) {
	// We always support an admin role (PostgreSQL 11+)

	// We support read/write roles on PostgreSQL 14+ only,
	// as support for predefined roles was added then.
	var supportsPredefinedRoles bool
	{
		var version string
		if err := conn.QueryRow(ctx, "SHOW server_version").Scan(&version); err != nil {
			return nil, fmt.Errorf("determine server version: %v", err)
		}
		c.log.Debug().Str("version", version).Msg("got postgres server version")

		major, _, _ := strings.Cut(version, ".")
		if n, err := strconv.Atoi(major); err != nil {
			return nil, fmt.Errorf("determine server version: %v", err)
		} else if n >= 14 {
			supportsPredefinedRoles = true
		}
	}

	// For legacy databases, just use the predefined admin role that we set up before.
	roles := EncoreRoles{st.Config.Superuser}
	if supportsPredefinedRoles {
		// Otherwise if we support predefined roles, add more roles to use.
		roles = append(roles,
			Role{RoleAdmin, "encore-admin", "admin"},
			Role{RoleWrite, "encore-write", "write"},
			Role{RoleRead, "encore-read", "read"},
		)
	}
	return roles, nil
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

// Setup sets up the given databases.
func (c *Cluster) Setup(ctx context.Context, appRoot string, md *meta.Data) error {
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
			db = c.initDB(svc.Name)
		}
		g.Go(func() error { return db.Setup(ctx, appRoot, svc, false, false) })
	}
	c.mu.Unlock()
	return g.Wait()
}

// SetupAndMigrate creates and migrates the given databases.
func (c *Cluster) SetupAndMigrate(ctx context.Context, appRoot string, md *meta.Data) error {
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
			db = c.initDB(svc.Name)
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

// Status reports the cluster's status.
func (c *Cluster) Status(ctx context.Context) (*ClusterStatus, error) {
	return c.driver.ClusterStatus(ctx, c.ID)
}

// Info reports information about a cluster.
func (c *Cluster) Info(ctx context.Context) (*ClusterInfo, error) {
	st, err := c.Start(ctx)
	if err != nil {
		return nil, err
	}

	info := &ClusterInfo{ClusterStatus: st}
	info.Encore = c.Roles
	return info, nil
}

// ClusterInfo returns information about a cluster.
type ClusterInfo struct {
	*ClusterStatus

	// Encore contains the roles to use to connect for an Encore app.
	// It is set if and only if the cluster is running.
	Encore EncoreRoles
}

// ConnURI reports the connection URI to connect to the given database
// in the cluster, authenticating with the given role.
func (s *ClusterStatus) ConnURI(database string, r Role) string {
	uri := fmt.Sprintf("user=%s password=%s dbname=%s", r.Username, r.Password, database)

	// Handle different ways of expressing the host
	cfg := s.Config
	if strings.HasPrefix(cfg.Host, "/") {
		uri += " host=" + cfg.Host // unix socket
	} else if host, port, err := net.SplitHostPort(cfg.Host); err == nil {
		uri += fmt.Sprintf(" host=%s port=%s", host, port) // host:port
	} else {
		uri += " host=" + cfg.Host // hostname
	}

	return uri
}

// EncoreRoles describes the credentials to use when connecting
// to the cluster as an Encore user.
type EncoreRoles []Role

func (roles EncoreRoles) Superuser() (Role, bool) { return roles.find(RoleSuperuser) }
func (roles EncoreRoles) Admin() (Role, bool)     { return roles.find(RoleAdmin) }
func (roles EncoreRoles) Write() (Role, bool)     { return roles.find(RoleWrite) }
func (roles EncoreRoles) Read() (Role, bool)      { return roles.find(RoleRead) }

func (roles EncoreRoles) First(typs ...RoleType) (Role, bool) {
	for _, typ := range typs {
		if r, ok := roles.find(typ); ok {
			return r, true
		}
	}
	return Role{}, false
}

func (roles EncoreRoles) find(typ RoleType) (Role, bool) {
	for _, r := range roles {
		if r.Type == typ {
			return r, true
		}
	}
	return Role{}, false
}

type RoleType string

const (
	RoleSuperuser RoleType = "superuser"
	RoleAdmin     RoleType = "admin"
	RoleWrite     RoleType = "write"
	RoleRead      RoleType = "read"
)

type Role struct {
	Type     RoleType
	Username string
	Password string
}

// quoteString quotes a string for use in SQL.
func quoteString(str string) string {
	return "'" + strings.ReplaceAll(str, "'", "''") + "'"
}
