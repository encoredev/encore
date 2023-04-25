package sqldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jackc/pgx/v5"
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
func (db *DB) Setup(ctx context.Context, appRoot string, dbMeta *meta.SQLDatabase, migrate, recreate bool) (err error) {
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
	if err := db.EnsureRoles(ctx, db.Cluster.Roles...); err != nil {
		return fmt.Errorf("ensure db roles %s: %v", db.Name, err)
	}
	if migrate || recreate || !db.migrated {
		if err := db.Migrate(ctx, appRoot, dbMeta); err != nil {
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
	adm, err := db.connectSuperuser(ctx)
	if err != nil {
		return err
	}
	defer adm.Close(context.Background())

	// Does it already exist?
	var dummy int
	err = adm.QueryRow(ctx, "SELECT 1 FROM pg_database WHERE datname = $1", db.Name).Scan(&dummy)
	owner, ok := db.Cluster.Roles.First(RoleAdmin, RoleSuperuser)
	if !ok {
		return errors.New("unable to find admin or superuser roles")
	}

	if err == pgx.ErrNoRows {
		db.log.Debug().Msg("creating database")
		// Sanitize names since this query does not support query params
		dbName := (pgx.Identifier{db.Name}).Sanitize()
		ownerName := (pgx.Identifier{owner.Username}).Sanitize()
		_, err = adm.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s OWNER %s;", dbName, ownerName))
	}
	if err != nil {
		db.log.Error().Err(err).Msg("failed to create database")
	}
	return err
}

// EnsureRoles ensures the roles have been granted access to this database.
func (db *DB) EnsureRoles(ctx context.Context, roles ...Role) error {
	adm, err := db.connectSuperuser(ctx)
	if err != nil {
		return err
	}
	defer adm.Close(context.Background())

	db.log.Debug().Msg("revoking public access")
	safeDBName := (pgx.Identifier{db.Name}).Sanitize()
	_, err = adm.Exec(ctx, "REVOKE ALL ON DATABASE "+safeDBName+" FROM public")
	if err != nil {
		return fmt.Errorf("revoke public: %v", err)
	}

	for _, role := range roles {
		var stmt string
		safeRoleName := (pgx.Identifier{role.Username}).Sanitize()
		switch role.Type {
		case RoleSuperuser:
			// Already granted; nothing to do
			continue
		case RoleAdmin:
			stmt = fmt.Sprintf("GRANT ALL ON DATABASE %s TO %s;", safeDBName, safeRoleName)
		case RoleWrite:
			stmt = fmt.Sprintf(`
				GRANT TEMP, CONNECT ON DATABASE %s TO %s;
				GRANT pg_read_all_data TO %s;
				GRANT pg_write_all_data TO %s;
			`, safeDBName, safeRoleName, safeRoleName, safeRoleName)
		case RoleRead:
			stmt = fmt.Sprintf(`
				GRANT TEMP, CONNECT ON DATABASE %s TO %s;
				GRANT pg_read_all_data TO %s;
			`, safeDBName, safeRoleName, safeRoleName)
		default:
			return fmt.Errorf("unknown role type %q", role.Type)
		}

		db.log.Debug().Str("role", role.Username).Str("db", db.Name).Msg("granting access to role")

		// We've observed race conditions in Postgres to grant access. Retry a few times.
		{
			var err error
			for i := 0; i < 5; i++ {
				_, err = adm.Exec(ctx, stmt)
				if err == nil {
					break
				}
				db.log.Debug().Str("role", role.Username).Str("db", db.Name).Err(err).Msg("error granting role, retrying")
				time.Sleep(250 * time.Millisecond)
			}
			if err != nil {
				return fmt.Errorf("grant %s role %s: %v", role.Type, role.Username, err)
			}
		}

		db.log.Debug().Str("role", role.Username).Str("db", db.Name).Msg("successfully granted access")
	}
	return nil
}

// Migrate migrates the database.
func (db *DB) Migrate(ctx context.Context, appRoot string, dbMeta *meta.SQLDatabase) (err error) {
	db.log.Debug().Msg("running database migrations")
	defer func() {
		if err != nil {
			db.log.Error().Err(err).Msg("migrations failed")
		} else {
			db.migrated = true
			db.log.Debug().Msg("migrations completed successfully")
		}
	}()

	info, err := db.Cluster.Info(ctx)
	if err != nil {
		return err
	} else if info.Status != Running {
		return errors.New("cluster not running")
	}

	admin, ok := info.Encore.First(RoleAdmin, RoleSuperuser)
	if !ok {
		return errors.New("unable to find superuser or admin roles")
	}
	uri := info.ConnURI(db.Name, admin)
	db.log.Debug().Str("uri", uri).Msg("running migrations")
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
		appRoot:           appRoot,
		migrationsRelPath: dbMeta.MigrationRelPath,
		migrations:        dbMeta.Migrations,
	}
	m, err := migrate.NewWithInstance("src", s, db.Name, instance)
	if err != nil {
		return err
	}

	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		db.log.Info().Msg("database already up to date")
		return nil
	}

	// If we have a dirty migration, reset the dirty flag and try again.
	// This is safe since all migrations run inside transactions.
	var dirty migrate.ErrDirty
	if errors.As(err, &dirty) {
		ver := dirty.Version - 1
		// golang-migrate uses -1 to mean "no version", not 0.
		if ver == 0 {
			ver = database.NilVersion
		}
		if err = m.Force(ver); err == nil {
			err = m.Up()
		}
	}

	// If we have removed a migration that failed to apply we can get an ErrNoChange error
	// after forcing the migration down to the previous version.
	if errors.Is(err, migrate.ErrNoChange) {
		db.log.Info().Msg("database already up to date")
		return nil
	} else if err != nil {
		return fmt.Errorf("could not migrate database %s: %v", db.Name, err)
	}
	db.log.Info().Msg("migration completed")
	return nil
}

// Drop drops the database in the cluster if it exists.
func (db *DB) Drop(ctx context.Context) error {
	adm, err := db.connectSuperuser(ctx)
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

// connectSuperuser creates a superuser connection to the root database for the cluster.
// On success the returned conn must be closed by the caller.
func (db *DB) connectSuperuser(ctx context.Context) (*pgx.Conn, error) {
	// Wait for the cluster to be setup
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-db.Cluster.started:
	}

	info, err := db.Cluster.Info(ctx)
	if err != nil {
		return nil, err
	} else if info.Status != Running {
		return nil, fmt.Errorf("cluster not running")
	}

	uri := info.ConnURI(info.Config.RootDatabase, info.Config.Superuser)

	// Wait for the connection to be established; this might take a little bit
	// when we're racing with spinning up a Docker container.
	for i := 0; i < 40; i++ {
		var conn *pgx.Conn
		conn, err = pgx.Connect(ctx, uri)
		if err == nil {
			return conn, nil
		} else if ctx.Err() != nil {
			// We'll never succeed once the context has been canceled.
			// Give up straight away.
			db.log.Debug().Err(err).Msgf("failed to connect to superuser db")
			return nil, err
		}
		time.Sleep(250 * time.Millisecond)
	}
	db.log.Debug().Err(err).Msgf("failed to connect to admin db")
	return nil, fmt.Errorf("failed to connect to superuser database: %v", err)
}
