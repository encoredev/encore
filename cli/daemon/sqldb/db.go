package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jackc/pgx/v4"
	"github.com/rs/zerolog"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

// DB represents a single database instance within a cluster.
type DB struct {
	Name    string // database name
	Cluster *Cluster

	// Ctx is canceled when the database is being torn down.
	Ctx    context.Context
	cancel func() // to cancel ctx

	setupMu sync.Mutex

	// ready is closed when the database is migrated and ready.
	ready   chan struct{}
	readied bool

	migrated bool

	log zerolog.Logger
}

// Ready returns a channel that is closed when the database is up and running.
func (db *DB) Ready() <-chan struct{} {
	return db.ready
}

// Setup sets up the database, (re)creating it if necessary and running schema migrations.
func (db *DB) Setup(ctx context.Context, appRoot string, svc *meta.Service, migrate, recreate bool) (err error) {
	db.log.Debug().Msg("setting up database")
	db.setupMu.Lock()
	defer db.setupMu.Unlock()
	defer func() {
		if err == nil {
			if !db.readied {
				db.readied = true
				close(db.ready)
			}
			db.log.Debug().Msg("successfully set up database")
		} else {
			db.log.Error().Err(err).Msg("failed to set up database")
		}
	}()

	if recreate {
		if err := db.Drop(ctx); err != nil {
			return fmt.Errorf("drop db %s: %v", db.Name, err)
		}
	}
	if err := db.Create(ctx); err != nil {
		return fmt.Errorf("create db %s: %v", db.Name, err)
	}
	if migrate || recreate || !db.migrated {
		if err := db.Migrate(ctx, appRoot, svc); err != nil {
			// Only report an error if we asked to migrate or recreate.
			// Otherwise we might fail to open a database shell when there
			// is a migration issue.
			if migrate || recreate {
				return fmt.Errorf("migrate db %s: %v", db.Name, err)
			}
		}
	}
	return nil
}

// Create creates the database in the cluster if it does not already exist.
// It reports whether the database was initialized for the first time
// in this process.
func (db *DB) Create(ctx context.Context) error {
	adm, err := db.connectAdminDB(ctx)
	if err != nil {
		return err
	}
	defer adm.Close(context.Background())

	// Does it already exist?
	var dummy int
	err = adm.QueryRow(ctx, "SELECT 1 FROM pg_database WHERE datname = $1", db.Name).Scan(&dummy)
	if err == pgx.ErrNoRows {
		db.log.Debug().Msg("creating database")
		name := (pgx.Identifier{db.Name}).Sanitize() // sanitize database name, to be safe
		_, err = adm.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s OWNER encore;", name))
	}
	if err != nil {
		db.log.Error().Err(err).Msg("failed to create database")
	}
	return err
}

// Migrate migrates the database.
func (db *DB) Migrate(ctx context.Context, appRoot string, svc *meta.Service) (err error) {
	db.log.Debug().Msg("running database migrations")
	defer func() {
		if err != nil {
			db.log.Error().Err(err).Msg("migrations failed")
		} else {
			db.migrated = true
			db.log.Debug().Msg("migrations completed successfully")
		}
	}()

	uri := fmt.Sprintf("postgresql://encore:%s@%s/%s?sslmode=disable", db.Cluster.ID, db.Cluster.HostPort, db.Name)
	conn, err := sql.Open("pgx", uri)
	if err != nil {
		return err
	}
	defer conn.Close()

	instance, err := postgres.WithInstance(conn, &postgres.Config{})
	if err != nil {
		return err
	}

	s := &src{
		appRoot:    appRoot,
		svcRelPath: svc.RelPath,
		migrations: svc.Migrations,
	}
	m, err := migrate.NewWithInstance("src", s, db.Name, instance)
	if err != nil {
		return err
	}

	if err := m.Up(); err == migrate.ErrNoChange {
		db.log.Debug().Msg("database already up to date")
		return nil
	} else if err != nil {
		return err
	}
	return nil
}

// Drop drops the database in the cluster if it exists.
func (db *DB) Drop(ctx context.Context) error {
	adm, err := db.connectAdminDB(ctx)
	if err != nil {
		return err
	}
	defer adm.Close(context.Background())

	var dummy int
	err = adm.QueryRow(ctx, "SELECT 1 FROM pg_database WHERE datname = $1", db.Name).Scan(&dummy)
	if err == nil {
		// Drop all connections to prevent "database is being accessed by other users" errors.
		adm.Exec(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", db.Name)

		name := (pgx.Identifier{db.Name}).Sanitize() // sanitize database name, to be safe
		_, err = adm.Exec(ctx, fmt.Sprintf("DROP DATABASE %s;", name))
		db.log.Debug().Err(err).Msgf("dropped database")
	} else if err == pgx.ErrNoRows {
		return nil
	}

	if err != nil {
		db.log.Debug().Err(err).Msgf("failed to drop database")
	}
	return err
}

// CloseConns closes all connections to this database through the dbproxy,
// and prevents future ones from being established.
func (db *DB) CloseConns() {
	db.cancel()
}

// connectAdminDB creates a connection to the admin database for the cluster.
// On success the returned conn must be closed by the caller.
func (db *DB) connectAdminDB(ctx context.Context) (*pgx.Conn, error) {
	// Wait for the cluster to be setup
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-db.Cluster.started:
	}

	hostPort := db.Cluster.HostPort
	if hostPort == "" {
		return nil, fmt.Errorf("internal error: missing HostPort for cluster %s", db.Cluster.ID)
	}

	// Wait for the connection to be established; this might take a little bit
	// when we're racing with spinning up a Docker container.
	var err error
	for i := 0; i < 40; i++ {
		var conn *pgx.Conn
		conn, err = pgx.Connect(ctx, "postgresql://encore:"+db.Cluster.ID+"@"+hostPort+"/postgres?sslmode=disable")
		if err == nil {
			return conn, nil
		} else if ctx.Err() != nil {
			// We'll never succeed once the context has been canceled.
			// Give up straight away.
			db.log.Debug().Err(err).Msgf("failed to connect to admin db")
			return nil, err
		}
		time.Sleep(250 * time.Millisecond)
	}
	db.log.Debug().Err(err).Msgf("failed to connect to admin db")
	return nil, fmt.Errorf("failed to connect to admin database: %v", err)
}
