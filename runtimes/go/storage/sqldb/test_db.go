package sqldb

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/xid"

	"encore.dev/appruntime/exported/config"
)

// migratorUser is the username the local dbproxy maps to the migrator
// role. We use it for CREATE/DROP DATABASE in test setup/teardown so
// those statements run with sufficient privileges; application queries
// continue to go through the regular service-user pool.
const migratorUser = "encore-migrator"

// superuserUser is the username the local dbproxy maps to the superuser
// role. Used for statements that require privileges beyond the migrator
// role (e.g. CREATE EXTENSION).
const superuserUser = "encore-superuser"

//publicapigen:drop
func (mgr *Manager) NewTestDatabase(ctx context.Context, name string) (*Database, error) {
	db := mgr.GetDB(name)
	if db.noopDB {
		return nil, fmt.Errorf("et: unknown database name: %q", name)
	}

	dbName := db.origName + "_" + xid.New().String()
	templateName := db.origName + "_template"

	if err := mgr.execAsMigrator(ctx, db.origName, fmt.Sprintf(
		"CREATE DATABASE %s TEMPLATE %s",
		pgx.Identifier{dbName}.Sanitize(),
		pgx.Identifier{templateName}.Sanitize(),
	)); err != nil {
		return nil, err
	}

	clone := &Database{
		name:     dbName,
		origName: db.origName,
		mgr:      mgr,
		hooks:    db.hooks,
	}

	mgr.ts.AddEndCallback(func(t *testing.T) {
		// Shut down the connection pools and attempt to drop the database.
		clone.shutdown()
		err := mgr.execAsMigrator(context.Background(), db.origName, fmt.Sprintf(
			"DROP DATABASE %s WITH (FORCE)",
			pgx.Identifier{dbName}.Sanitize(),
		))
		if err != nil {
			mgr.rootLogger.Error().Err(err).Str("database", dbName).Msg("failed to clean up test database")
		}
	})
	return clone, nil
}

// execAsMigrator opens a one-shot connection to the local dbproxy as
// the migrator user and runs the given statement. The proxy maps the
// "encore-migrator" username onto the migrator role; the password
// stays the cluster password (already set on the regular DB config).
func (mgr *Manager) execAsMigrator(ctx context.Context, dbEncoreName, stmt string) error {
	var dbCfg *config.SQLDatabase
	for _, d := range mgr.runtime.SQLDatabases {
		if d.EncoreName == dbEncoreName {
			dbCfg = d
			break
		}
	}
	if dbCfg == nil {
		return fmt.Errorf("sqldb: unknown database %q", dbEncoreName)
	}
	srv := mgr.runtime.SQLServers[dbCfg.ServerID]

	// Reuse the regular pool config (TLS, host parsing, etc.), then
	// swap the user to the migrator. Password stays unchanged — the
	// dbproxy authenticates with the cluster password regardless of
	// the requested role.
	poolCfg, err := dbConf(srv, dbCfg, "")
	if err != nil {
		return fmt.Errorf("sqldb: build migrator conn config: %w", err)
	}
	connCfg := poolCfg.ConnConfig.Copy()
	connCfg.User = migratorUser

	conn, err := pgx.ConnectConfig(ctx, connCfg)
	if err != nil {
		return fmt.Errorf("sqldb: connect as migrator: %w", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()

	if _, err := conn.Exec(ctx, stmt); err != nil {
		return err
	}
	return nil
}

// WithSuperuser returns a copy of db whose connections to the local
// dbproxy authenticate as the superuser. The returned database shares
// the same hooks as the original and is automatically shut down at the
// end of the test.
//
// Use this for operations that require superuser privileges (e.g.
// CREATE EXTENSION) during test setup.
//
//publicapigen:drop
func (mgr *Manager) WithSuperuser(db *Database) *Database {
	if db.noopDB {
		return db
	}

	var dbCfg *config.SQLDatabase
	for _, d := range mgr.runtime.SQLDatabases {
		if d.EncoreName == db.origName {
			dbCfg = d
			break
		}
	}
	if dbCfg == nil {
		panic(fmt.Sprintf("sqldb: unknown database %q", db.origName))
	}
	srv := mgr.runtime.SQLServers[dbCfg.ServerID]

	// For a cloned test database db.name is the actual Postgres name;
	// for a regular database it equals the encore name and we let
	// dbConf fall back to dbCfg.DatabaseName.
	dbNameOverride := ""
	if db.name != db.origName {
		dbNameOverride = db.name
	}

	cfg, err := dbConf(srv, dbCfg, dbNameOverride)
	if err != nil {
		panic("sqldb: " + err.Error())
	}
	cfg.ConnConfig.User = superuserUser
	cfg.ConnConfig.Tracer = &pgxTracer{mgr: mgr}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return db.hooks.runAfterConnectHooks(ctx, conn)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		panic("sqldb: setup superuser db: " + err.Error())
	}

	clone := &Database{
		name:     db.name,
		origName: db.origName,
		mgr:      mgr,
		hooks:    db.hooks,
		pool:     pool,
	}
	mgr.ts.AddEndCallback(func(t *testing.T) {
		clone.shutdown()
	})
	return clone
}
