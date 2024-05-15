package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// DB represents a single database instance within a cluster.
type DB struct {
	EncoreName string
	Cluster    *Cluster

	driverName string

	// Ctx is canceled when the database is being torn down.
	Ctx    context.Context
	cancel func() // to cancel ctx

	setupMu sync.Mutex

	// ready is closed when the database is migrated and ready.
	ready   chan struct{}
	readied bool

	migrated bool

	// template indicates the database is backed by a template database.
	template bool

	log zerolog.Logger
}

// ApplicationCloudName reports the "cloud name" of the application-facing database.
func (db *DB) ApplicationCloudName() string {
	return db.driverName
}

// TemplateCloudName reports the "cloud name" of the template database, if any.
func (db *DB) TemplateCloudName() option.Option[string] {
	if db.template {
		return option.Some(db.driverName + "_template")
	}
	return option.None[string]()
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
		if err := db.drop(ctx); err != nil {
			return err
		}
	}

	setupDB := func(cloudName string) error {
		if err := db.doCreate(ctx, cloudName, option.None[string]()); err != nil {
			return errors.Wrapf(err, "create db %s: %v", cloudName, err)
		}

		if err := db.ensureRoles(ctx, cloudName, db.Cluster.Roles...); err != nil {
			return fmt.Errorf("ensure db roles %s: %v", cloudName, err)
		}

		if migrate || recreate || !db.migrated {
			if err := db.doMigrate(ctx, cloudName, appRoot, dbMeta); err != nil {
				// Only report an error if we asked to migrate or recreate.
				// Otherwise we might fail to open a database shell when there
				// is a migration issue.
				if migrate || recreate {
					return fmt.Errorf("migrate db %s: %v", cloudName, err)
				}
			}
		}
		return nil
	}

	// First set up the database with the application name.
	if err := setupDB(db.ApplicationCloudName()); err != nil {
		return err
	}

	if tmplName, ok := db.TemplateCloudName().Get(); ok {
		// If we want a template database, rename the application database to the template name.
		// We do it this way in case the migrations assume the database is named according to the application name.

		// Terminate the connections to the template database to prevent "database is being accessed by other users" errors.
		_ = db.terminateConnectionsToDB(ctx, db.ApplicationCloudName())
		if err := db.renameDB(ctx, db.ApplicationCloudName(), tmplName); err != nil {
			return fmt.Errorf("rename db %s to %s: %v", db.ApplicationCloudName(), tmplName, err)
		}

		// Then create the application database based on the template
		if err := db.doCreate(ctx, db.ApplicationCloudName(), option.Some(tmplName)); err != nil {
			return errors.Wrapf(err, "create db %s: %v", db.ApplicationCloudName(), err)
		}

		// Ensure the application database has the right roles, too.
		if err := db.ensureRoles(ctx, db.ApplicationCloudName(), db.Cluster.Roles...); err != nil {
			return fmt.Errorf("ensure db roles %s: %v", db.ApplicationCloudName(), err)
		}
	}

	return nil
}

func (db *DB) doCreate(ctx context.Context, cloudName string, template option.Option[string]) error {
	adm, err := db.connectSuperuser(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = adm.Close(context.Background()) }()

	// Does it already exist?
	var dummy int
	err = adm.QueryRow(ctx, "SELECT 1 FROM pg_database WHERE datname = $1", cloudName).Scan(&dummy)
	owner, ok := db.Cluster.Roles.First(RoleAdmin, RoleSuperuser)
	if !ok {
		return errors.New("unable to find admin or superuser roles")
	}

	if errors.Is(err, pgx.ErrNoRows) {
		db.log.Debug().Msg("creating database")
		// Sanitize names since this query does not support query params
		dbName := (pgx.Identifier{cloudName}).Sanitize()
		ownerName := (pgx.Identifier{owner.Username}).Sanitize()

		// Use the template if one is provided.
		var tmplSnippet string
		if tmplName, ok := template.Get(); ok {
			tmplSnippet = fmt.Sprintf("WITH TEMPLATE %s", (pgx.Identifier{tmplName}).Sanitize())
		}
		_, err = adm.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s %s OWNER %s;", dbName, tmplSnippet, ownerName))
	}
	if err != nil {
		db.log.Error().Err(err).Msg("failed to create database")
	}
	return err
}

func (db *DB) renameDB(ctx context.Context, from, to string) error {
	adm, err := db.connectSuperuser(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = adm.Close(context.Background()) }()

	_, err = adm.Exec(ctx, fmt.Sprintf("ALTER DATABASE %s RENAME TO %s",
		(pgx.Identifier{from}).Sanitize(),
		(pgx.Identifier{to}).Sanitize(),
	))
	return err
}

// ensureRoles ensures the roles have been granted access to this database.
func (db *DB) ensureRoles(ctx context.Context, cloudName string, roles ...Role) error {
	adm, err := db.connectSuperuser(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = adm.Close(context.Background()) }()

	db.log.Debug().Msg("revoking public access")
	safeDBName := (pgx.Identifier{cloudName}).Sanitize()
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

		db.log.Debug().Str("role", role.Username).Str("db", cloudName).Msg("granting access to role")

		// We've observed race conditions in Postgres to grant access. Retry a few times.
		{
			var err error
			for i := 0; i < 5; i++ {
				_, err = adm.Exec(ctx, stmt)
				if err == nil {
					break
				}
				db.log.Debug().Str("role", role.Username).Str("db", cloudName).Err(err).Msg("error granting role, retrying")
				time.Sleep(250 * time.Millisecond)
			}
			if err != nil {
				return fmt.Errorf("grant %s role %s: %v", role.Type, role.Username, err)
			}
		}

		db.log.Debug().Str("role", role.Username).Str("db", cloudName).Msg("successfully granted access")
	}
	return nil
}

// Migrate migrates the database.
func (db *DB) doMigrate(ctx context.Context, cloudName, appRoot string, dbMeta *meta.SQLDatabase) (err error) {
	if db.Cluster.ID.Type == Shadow {
		db.log.Debug().Msg("not applying migrations to shadow cluster")
		return nil
	}
	if len(dbMeta.Migrations) == 0 || dbMeta.MigrationRelPath == nil {
		db.log.Debug().Msg("no database migrations to run, skipping")
		return nil
	}

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
	uri := info.ConnURI(cloudName, admin)
	db.log.Debug().Str("uri", uri).Msg("running migrations")
	conn, err := sql.Open("pgx", uri)
	if err != nil {
		return err
	}
	defer fns.CloseIgnore(conn)

	instance, err := postgres.WithInstance(conn, &postgres.Config{})
	if err != nil {
		return err
	}

	s := &src{
		appRoot:           appRoot,
		migrationsRelPath: *dbMeta.MigrationRelPath,
		migrations:        dbMeta.Migrations,
	}
	m, err := migrate.NewWithInstance("src", s, cloudName, instance)
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
		// Find the version that preceded the dirty version so
		// we can force the migration to that version and then
		// re-apply the migration.
		var prevVer uint
		prevVer, err = s.Prev(uint(dirty.Version))
		targetVer := int(prevVer)
		if errors.Is(err, fs.ErrNotExist) {
			// No previous migration exists
			targetVer = database.NilVersion
		} else if err != nil {
			return errors.Wrap(err, "failed to find previous version")
		}

		if err = m.Force(targetVer); err == nil {
			err = m.Up()
		}
	}

	// If we have removed a migration that failed to apply we can get an ErrNoChange error
	// after forcing the migration down to the previous version.
	if errors.Is(err, migrate.ErrNoChange) {
		db.log.Info().Msg("database already up to date")
		return nil
	} else if err != nil {
		return fmt.Errorf("could not migrate database %s: %v", cloudName, err)
	}
	db.log.Info().Msg("migration completed")
	return nil
}

func (db *DB) drop(ctx context.Context) error {
	if err := db.doDrop(ctx, db.ApplicationCloudName()); err != nil {
		return errors.Wrapf(err, "drop database %s", db.ApplicationCloudName())
	}
	if name, ok := db.TemplateCloudName().Get(); ok {
		if err := db.doDrop(ctx, name); err != nil {
			return errors.Wrapf(err, "drop database %s", name)
		}
	}
	return nil
}

func (db *DB) terminateConnectionsToDB(ctx context.Context, cloudName string) error {
	adm, err := db.connectSuperuser(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = adm.Close(context.Background()) }()

	// Drop all connections to prevent "database is being accessed by other users" errors.
	_, _ = adm.Exec(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", cloudName)
	return nil
}

func (db *DB) doDrop(ctx context.Context, cloudName string) error {
	adm, err := db.connectSuperuser(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = adm.Close(context.Background()) }()

	var dummy int
	err = adm.QueryRow(ctx, "SELECT 1 FROM pg_database WHERE datname = $1", cloudName).Scan(&dummy)
	if err == nil {
		// Drop all connections to prevent "database is being accessed by other users" errors.
		_, _ = adm.Exec(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", cloudName)

		name := (pgx.Identifier{cloudName}).Sanitize() // sanitize database name, to be safe
		_, err = adm.Exec(ctx, fmt.Sprintf("DROP DATABASE %s;", name))
		db.log.Debug().Err(err).Msgf("dropped database")
	} else if errors.Is(err, pgx.ErrNoRows) {
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
