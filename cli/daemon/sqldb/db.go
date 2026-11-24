package sqldb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"

	"encr.dev/cli/daemon/internal/debugflags"
	"encr.dev/internal/version"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// migratorRoles returns the role precedence to use when running migrations
// or creating databases. ENCOREDEBUG=sqldbrole=legacy reverts to the
// pre-migrator-role behavior of running migrations as the admin role.
func migratorRoles() []RoleType {
	if debugflags.Get("sqldbrole") == debugflags.SQLDBRoleLegacy {
		return []RoleType{RoleAdmin, RoleSuperuser}
	}
	return []RoleType{RoleMigrator, RoleAdmin, RoleSuperuser}
}

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
func (db *DB) Setup(ctx context.Context, appRoot string, dbMeta *meta.SQLDatabase, migrate, recreate bool) error {
	return db.setup(ctx, appRoot, dbMeta, setupOptions{migrate: migrate, recreate: recreate})
}

// RecreateFromTemplate recreates the database like Setup with recreate set, except that it only copies
// the template database when the template already holds the current migrations.
func (db *DB) RecreateFromTemplate(ctx context.Context, appRoot string, dbMeta *meta.SQLDatabase) error {
	return db.setup(ctx, appRoot, dbMeta, setupOptions{migrate: true, recreate: true, reuseTemplate: true})
}

type setupOptions struct {
	migrate       bool
	recreate      bool
	reuseTemplate bool // with recreate, copy a template that already holds the current migrations
}

func (db *DB) setup(ctx context.Context, appRoot string, dbMeta *meta.SQLDatabase, opts setupOptions) (err error) {
	migrate, recreate := opts.migrate, opts.recreate
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

	tmplName, useTemplate := db.TemplateCloudName().Get()
	var tmplKey string
	if recreate && useTemplate {
		var keyErr error
		tmplKey, keyErr = templateKey(appRoot, dbMeta)
		if keyErr != nil {
			// doMigrate reports an unreadable migration below; here it only means the template is rebuilt.
			db.log.Debug().Err(keyErr).Msg("could not hash the migrations")
		}
		if opts.reuseTemplate && templateReuseEnabled() && tmplKey != "" && db.storedTemplateKey(ctx, tmplName) == tmplKey {
			if err := db.copyTemplate(ctx, tmplName); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				db.log.Warn().Err(err).Msg("could not copy the template database, rebuilding it")
			} else {
				db.migrated = true
				db.log.Debug().Msg("copied the database from its template")
				return nil
			}
		}
	}

	if recreate {
		if err := db.drop(ctx); err != nil {
			return fmt.Errorf("drop database: %v", err)
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

	if useTemplate {
		// If we want a template database, rename the application database to the template name.
		// We do it this way in case the migrations assume the database is named according to the application name.

		// Terminate the connections to the template database to prevent "database is being accessed by other users" errors.
		_ = db.terminateConnectionsToDB(ctx, db.ApplicationCloudName())
		if err := db.doDrop(ctx, tmplName); err != nil {
			return fmt.Errorf("drop db %s: %v", tmplName, err)
		}

		if err := db.renameDB(ctx, db.ApplicationCloudName(), tmplName); err != nil {
			return fmt.Errorf("rename db %s to %s: %v", db.ApplicationCloudName(), tmplName, err)
		}

		// Record what the template was built from, so a later recreate can copy it. Only a recreate
		// starts from an empty database and fails on a migration error, so only then does the template
		// hold exactly these migrations and no rows from earlier runs.
		// A migration edited while it was applied would leave the template out of step with the key.
		if after, _ := templateKey(appRoot, dbMeta); tmplKey != "" && after == tmplKey {
			db.setTemplateKey(ctx, tmplName, tmplKey)
		}

		return db.createFromTemplate(ctx, tmplName)
	}

	return nil
}

// copyTemplate replaces the application database with a copy of the template.
func (db *DB) copyTemplate(ctx context.Context, tmplName string) error {
	if err := db.doDrop(ctx, db.ApplicationCloudName()); err != nil {
		return fmt.Errorf("drop db %s: %v", db.ApplicationCloudName(), err)
	}
	// A connection to the template makes copying it fail with "database is being accessed by other users".
	_ = db.terminateConnectionsToDB(ctx, tmplName)
	return db.createFromTemplate(ctx, tmplName)
}

// createFromTemplate creates the application database as a copy of the template.
func (db *DB) createFromTemplate(ctx context.Context, tmplName string) error {
	if err := db.doCreate(ctx, db.ApplicationCloudName(), option.Some(tmplName)); err != nil {
		return errors.Wrapf(err, "create db %s: %v", db.ApplicationCloudName(), err)
	}

	// Ensure the application database has the right roles, too.
	if err := db.ensureRoles(ctx, db.ApplicationCloudName(), db.Cluster.Roles...); err != nil {
		return fmt.Errorf("ensure db roles %s: %v", db.ApplicationCloudName(), err)
	}
	return nil
}

// templateKeyPrefix marks the comment on a template database that holds its key.
const templateKeyPrefix = "encore-template:"

// templateFormat is part of every template key. Bump it when a change to how databases are
// migrated or set up should make every existing template stale.
const templateFormat = 1

// templateReuseEnabled reports whether encore test may copy a template instead of migrating again.
// ENCOREDEBUG=sqldbtemplate=rebuild turns it off, for example for tests that need seed rows from a fresh migration.
func templateReuseEnabled() bool {
	return debugflags.Get("sqldbtemplate") != debugflags.SQLDBTemplateRebuild
}

// templateKey identifies what a template database is built from: the migration files, and the Encore
// version, template format and settings that change how they are applied. It is "" without migrations.
func templateKey(appRoot string, dbMeta *meta.SQLDatabase) (string, error) {
	migrations, err := HashMigrations(appRoot, dbMeta)
	if err != nil || migrations == "" {
		return "", err
	}
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%d\x00%s\x00%s\x00%v\x00%t", templateFormat, version.Version, migrations, migratorRoles(), dbMeta.AllowNonSequentialMigrations)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// storedTemplateKey returns the key recorded on the template database, or "" if there is none.
func (db *DB) storedTemplateKey(ctx context.Context, tmplName string) string {
	adm, err := db.connectSuperuser(ctx)
	if err != nil {
		db.log.Debug().Err(err).Msg("could not read the template key")
		return ""
	}
	defer func() { _ = adm.Close(context.Background()) }()

	// CREATE DATABASE ... TEMPLATE does not copy comments, so a template replaced any other way has none.
	var comment *string
	err = adm.QueryRow(ctx, "SELECT shobj_description(oid, 'pg_database') FROM pg_database WHERE datname = $1", tmplName).Scan(&comment)
	if err != nil {
		db.log.Debug().Err(err).Msg("could not read the template key")
		return ""
	}
	if comment == nil {
		return ""
	}
	key, ok := strings.CutPrefix(*comment, templateKeyPrefix)
	if !ok {
		return ""
	}
	return key
}

// setTemplateKey records key on the template database. Failing only costs the next reuse.
func (db *DB) setTemplateKey(ctx context.Context, tmplName, key string) {
	adm, err := db.connectSuperuser(ctx)
	if err != nil {
		db.log.Warn().Err(err).Msg("could not record the template key")
		return
	}
	defer func() { _ = adm.Close(context.Background()) }()

	// COMMENT takes no parameters. The key is hex, so quoting it is enough.
	stmt := fmt.Sprintf("COMMENT ON DATABASE %s IS %s", (pgx.Identifier{tmplName}).Sanitize(), quoteString(templateKeyPrefix+key))
	if _, err := adm.Exec(ctx, stmt); err != nil {
		db.log.Warn().Err(err).Msg("could not record the template key")
	}
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
	owner, ok := db.Cluster.Roles.First(migratorRoles()...)
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
//
// Cluster-wide role memberships (GRANT pg_read_all_data, encore_services, etc.)
// are applied once in Cluster.setupRoles. This function handles only the
// database-local statements, but still serializes on Cluster.rolesMu because
// REASSIGN OWNED and GRANT ... ON DATABASE touch role rows in pg_authid and
// can deadlock when run concurrently across databases in the same cluster.
func (db *DB) ensureRoles(ctx context.Context, cloudName string, roles ...Role) error {
	adm, err := db.connectToDB(ctx)
	if err != nil {
		return fmt.Errorf("connect to db: %v", err)
	}
	defer func() { _ = adm.Close() }()

	db.Cluster.rolesMu.Lock()
	defer db.Cluster.rolesMu.Unlock()

	db.log.Debug().Msg("revoking public access")
	safeDBName := (pgx.Identifier{cloudName}).Sanitize()
	if _, err := adm.ExecContext(ctx, "REVOKE ALL ON DATABASE "+safeDBName+" FROM public"); err != nil {
		return fmt.Errorf("revoke public: %v", err)
	}

	for _, role := range roles {
		var stmt string
		safeRoleName := (pgx.Identifier{role.Username}).Sanitize()
		switch role.Type {
		case RoleSuperuser:
			// Already granted; nothing to do
			continue
		case RoleService:
			// Only cluster-wide memberships; nothing to do per-database.
			continue
		case RoleServices, RoleMigrator:
			stmt = fmt.Sprintf(`GRANT ALL ON DATABASE %s TO %s;`, safeDBName, safeRoleName)
		case RoleAdmin:
			stmt = fmt.Sprintf(`
				GRANT ALL ON DATABASE %[1]s TO %[2]s;
				REASSIGN OWNED BY %[2]s TO "encore-migrator"`,
				safeDBName, safeRoleName)
		case RoleWrite, RoleRead:
			stmt = fmt.Sprintf(`GRANT TEMP, CONNECT ON DATABASE %s TO %s;`, safeDBName, safeRoleName)
		default:
			return fmt.Errorf("unknown role type %q", role.Type)
		}

		db.log.Debug().Str("role", role.Username).Str("db", cloudName).Msg("granting access to role")

		if _, err := adm.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("grant %s role %s: %v", role.Type, role.Username, err)
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

	admin, ok := info.Encore.First(migratorRoles()...)
	if !ok {
		return errors.New("unable to find superuser or admin roles")
	}
	uri := info.ConnURI(cloudName, admin)
	db.log.Debug().Str("uri", uri).Msg("running migrations")
	pool, err := sql.Open("pgx", uri)
	if err != nil {
		return err
	}
	defer fns.CloseIgnore(pool)

	path := filepath.Join(appRoot, *dbMeta.MigrationRelPath)
	mdSrc := NewMetadataSource(NewOsMigrationReader(path), dbMeta.Migrations)
	conn, err := pool.Conn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to postgres")
	}
	err = RunMigration(ctx, cloudName, dbMeta.AllowNonSequentialMigrations, conn, mdSrc)

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

func (db *DB) ListAppliedMigrations(ctx context.Context) (map[uint64]bool, error) {
	conn, err := db.connectToDB(ctx)
	if err != nil {
		return nil, err
	}
	defer fns.CloseIgnore(conn)
	return LoadAppliedVersions(ctx, conn, "public", "schema_migrations")
}

// PendingMigrations returns the migrations in dbMeta that have not been applied,
// given the applied versions as returned by ListAppliedMigrations (version -> dirty).
// A dirty version counts as not applied.
func PendingMigrations(dbMeta *meta.SQLDatabase, applied map[uint64]bool) []*meta.DBMigration {
	var pending []*meta.DBMigration
	if dbMeta.AllowNonSequentialMigrations {
		for _, m := range dbMeta.Migrations {
			if dirty, ok := applied[m.Number]; !ok || dirty {
				pending = append(pending, m)
			}
		}
		return pending
	}

	// Sequential migrations only track the latest version,
	// so everything after it is pending.
	if len(applied) == 0 {
		return dbMeta.Migrations
	}
	var latest uint64
	for v := range applied {
		latest = max(latest, v)
	}
	for _, m := range dbMeta.Migrations {
		if m.Number > latest || (m.Number == latest && applied[latest]) {
			pending = append(pending, m)
		}
	}
	return pending
}

func RunMigration(ctx context.Context, dbName string, allowNonSeq bool, conn *sql.Conn, mdSrc *MetadataSource) (err error) {
	var (
		dbDriver  database.Driver
		srcDriver source.Driver
	)
	if allowNonSeq {
		dbDriver, srcDriver, err = NonSequentialMigrator(ctx, conn, mdSrc)
		if err != nil {
			return errors.Wrap(err, "failed to connect to postgres")
		}
	} else {
		dbDriver, err = postgres.WithConnection(ctx, conn, &postgres.Config{})
		if err != nil {
			return errors.Wrap(err, "failed to connect to postgres")
		}
		srcDriver = mdSrc
	}

	curVersion, _, err := dbDriver.Version()
	if err != nil {
		return errors.Wrap(err, "failed to get current version")
	} else if curVersion < -1 {
		return errors.Newf("invalid current version (%d) for db %s", curVersion, dbName)
	}

	m, err := migrate.NewWithInstance("src", srcDriver, "postgres", dbDriver)
	if err != nil {
		return errors.Wrap(err, "failed to create migration instance")
	}

	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	// If we have a dirty migration, reset the dirty flag and try again.
	// This is safe since all migrations run inside transactions.
	var dirty migrate.ErrDirty
	if errors.As(err, &dirty) {
		// Find the version that preceded the dirty version so
		// we can force the migration to that version and then
		// re-apply the migration.
		var prevVer uint
		prevVer, err = srcDriver.Prev(uint(dirty.Version))
		targetVer := int(prevVer)
		if errors.Is(err, fs.ErrNotExist) {
			// If Prev returns ErrNotExist, the original migration might
			// have been deleted. In this case, we'll need to search for
			// the version that is the closest lower version starting at the
			// first version.
			targetVer, err = findClosestLowerVersion(srcDriver.First, dirty.Version, srcDriver.Next)
			if err != nil {
				return errors.Wrapf(err, "could not automatically reset the schema_migrations "+
					"dirty flag for database %s. Please reset it manually by connecting "+
					"to the database modify the schema_migrations table", dbName)
			}
		} else if err != nil {
			return errors.Wrap(err, "failed to find previous version")
		}

		if err = m.Force(targetVer); err == nil {
			err = m.Up()
		}
	}
	return errors.Wrap(err, "failed to migrate database")
}

func findClosestLowerVersion(first func() (uint, error), dirtyVer int, next func(i uint) (uint, error)) (int, error) {
	firstVer, err := first()
	// If the first version doesn't exist, we can't reset the dirty flag
	// and we'll need to return an error.
	if err != nil {
		return 0, errors.Wrapf(err, "failed to find first version")
	}
	// otherwise we'll need to find the version that is the closest lower version
	rtn := database.NilVersion
	for nextVer := firstVer; err == nil && nextVer < uint(dirtyVer); nextVer, err = next(nextVer) {
		rtn = int(nextVer)
	}
	return rtn, nil
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
		return fmt.Errorf("connect as superuser: %v", err)
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
	for range 40 {
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

// Connects as a superuser or admin to the database. Fails fast if the cluster
// is not running yet.
// On success the returned conn must be closed by the caller.
func (db *DB) connectToDB(ctx context.Context) (*sql.Conn, error) {
	info, err := db.Cluster.Info(ctx)
	if err != nil {
		return nil, err
	}
	uri := info.ConnURI(db.ApplicationCloudName(), info.Config.Superuser)
	pool, err := sql.Open("pgx", uri)
	if err != nil {
		return nil, err
	}
	defer fns.CloseIgnore(pool)

	conn, err := pool.Conn(ctx)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
