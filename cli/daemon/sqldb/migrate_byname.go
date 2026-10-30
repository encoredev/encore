package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/lib/pq"

	"encr.dev/cli/daemon/internal/debugflags"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// Name-based migration tracking for local development.
//
// Upstream Encore tracks applied migrations golang-migrate style, by number,
// in the schema_migrations table. That breaks on developer machines when
// migration files get renumbered: if two branches both add 1002_*, and the
// later one is renumbered to 1003_* before merging, a developer who already
// applied the old 1002_* locally has schema_migrations.version=1002, so the
// other branch's 1002_* is never applied and the renumbered file is applied
// a second time as 1003_*.
//
// To fix this the local daemon tracks applied migrations by name (the part
// of the filename after the number prefix) in the encore_schema_migrations
// table. Migration files are still applied in numeric order, but every file
// whose name has not been applied yet is applied, even when its number is
// lower than the highest applied number, and a renumbered file whose name has
// already been applied is skipped.
//
// The legacy schema_migrations table is still kept up to date (rewritten from
// the name table after every run) so that switching back to an upstream
// Encore CLI keeps working.
//
// This code only runs in the local daemon (encore run, encore test, encore db
// and the local dashboard). Deployed environments are migrated by Encore
// Cloud or by the user's own tooling and are not affected.
//
// Set ENCOREDEBUG=migrations=legacy (before the daemon starts) to restore the
// upstream number-based behavior.

const (
	// byNameMigrationsTable is the table that tracks applied migrations by name.
	byNameMigrationsTable = "encore_schema_migrations"
	// legacyMigrationsTable is the golang-migrate table used by upstream Encore.
	legacyMigrationsTable = "schema_migrations"

	// migrationsDebugFlag is the ENCOREDEBUG flag that selects the tracking mode.
	migrationsDebugFlag = "migrations"
	// migrationsLegacy is the value of the migrations flag that restores
	// upstream number-based tracking.
	migrationsLegacy = "legacy"

	// byNameAdvisoryLockID is the pg_advisory_lock key used while migrating.
	// Advisory locks are scoped to the current database.
	byNameAdvisoryLockID = 0x656e636f72656d67 // "encoremg"
)

// trackMigrationsByName reports whether the daemon tracks local migrations by name.
// It's a variable so tests can override it.
var trackMigrationsByName = func() bool {
	return debugflags.Get(migrationsDebugFlag) != migrationsLegacy
}

// AppliedMigrationsAreASet reports whether the applied migrations returned by
// ListAppliedMigrations for dbMeta list every applied migration individually,
// as opposed to only the latest applied version (golang-migrate's sequential mode).
func AppliedMigrationsAreASet(dbMeta *meta.SQLDatabase) bool {
	return dbMeta.AllowNonSequentialMigrations || trackMigrationsByName()
}

var migrationNumberPrefix = regexp.MustCompile(`^\d+_?`)

// migrationName returns the name used to identify a migration across renumbering:
// the filename without its number prefix and its .up.sql/.sql suffix.
// For example "1002_add_users.up.sql" becomes "add_users".
func migrationName(m *meta.DBMigration) string {
	name := m.Filename
	name = strings.TrimSuffix(name, ".up.sql")
	name = strings.TrimSuffix(name, ".sql")
	name = migrationNumberPrefix.ReplaceAllString(name, "")
	if name == "" {
		// A file without a description (e.g. "3.up.sql") has nothing that
		// survives renumbering, so fall back to the full filename.
		return m.Filename
	}
	return name
}

// sortedMigrations returns the migrations sorted by number.
func sortedMigrations(migs []*meta.DBMigration) []*meta.DBMigration {
	out := slices.Clone(migs)
	slices.SortStableFunc(out, func(a, b *meta.DBMigration) int {
		switch {
		case a.Number < b.Number:
			return -1
		case a.Number > b.Number:
			return 1
		}
		return 0
	})
	return out
}

// migrationKeys returns the tracking key of each migration in migs, which must be
// sorted by number. Names that occur more than once get an occurrence suffix
// ("add_index", "add_index#2", ...), which survives renumbering as long as the
// relative order of the duplicates doesn't change.
func migrationKeys(migs []*meta.DBMigration) []string {
	keys := make([]string, len(migs))
	seen := make(map[string]int, len(migs))
	for i, m := range migs {
		name := migrationName(m)
		seen[name]++
		if n := seen[name]; n > 1 {
			name = fmt.Sprintf("%s#%d", name, n)
		}
		keys[i] = name
	}
	return keys
}

// pendingByName returns the indices (into migs/keys) of migrations that need to
// be applied, in order. A migration is pending if its key hasn't been applied,
// or if it has been applied but is marked dirty.
func pendingByName(keys []string, applied map[string]bool) []int {
	var pending []int
	for i, key := range keys {
		if dirty, ok := applied[key]; !ok || dirty {
			pending = append(pending, i)
		}
	}
	return pending
}

// backfillFromLegacy computes the name-based state for a database that has only
// been migrated with number-based tracking, using the current migration files to
// map numbers to names. legacy is the content of schema_migrations (version -> dirty).
//
// In sequential mode schema_migrations has a single row with the latest applied
// version: everything numbered below it is applied, the version itself is applied
// unless dirty, and everything above it is pending. In non-sequential mode each
// row is an applied (or dirty) version.
func backfillFromLegacy(migs []*meta.DBMigration, keys []string, legacy map[uint64]bool, allowNonSeq bool) map[string]bool {
	applied := make(map[string]bool)
	if len(legacy) == 0 {
		return applied
	}
	if allowNonSeq {
		for i, m := range migs {
			if dirty, ok := legacy[m.Number]; ok {
				applied[keys[i]] = dirty
			}
		}
		return applied
	}

	var (
		latest uint64
		dirty  bool
	)
	for v, d := range legacy {
		if v >= latest {
			latest, dirty = v, d
		}
	}
	for i, m := range migs {
		switch {
		case m.Number < latest:
			applied[keys[i]] = false
		case m.Number == latest:
			applied[keys[i]] = dirty
		}
	}
	return applied
}

// appliedNumbers maps the name-based state onto the current migration numbers
// (number -> dirty). Applied names that no longer have a file are ignored.
func appliedNumbers(migs []*meta.DBMigration, keys []string, applied map[string]bool) map[uint64]bool {
	out := make(map[uint64]bool, len(applied))
	for i, m := range migs {
		if dirty, ok := applied[keys[i]]; ok {
			out[m.Number] = dirty
		}
	}
	return out
}

func qualifiedTable(schema, table string) string {
	return pq.QuoteIdentifier(schema) + "." + pq.QuoteIdentifier(table)
}

func currentSchema(ctx context.Context, conn *sql.Conn) (string, error) {
	var schema string
	if err := conn.QueryRowContext(ctx, `SELECT CURRENT_SCHEMA()`).Scan(&schema); err != nil {
		return "", errors.Wrap(err, "failed to get current schema")
	} else if schema == "" {
		return "", errors.New("no current schema")
	}
	return schema, nil
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func tableExists(ctx context.Context, q queryer, schema, table string) (bool, error) {
	var exists bool
	err := q.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, qualifiedTable(schema, table)).Scan(&exists)
	return exists, errors.Wrapf(err, "failed to check for table %s", table)
}

func loadByName(ctx context.Context, q queryer, schema string) (map[string]bool, error) {
	rows, err := q.QueryContext(ctx, `SELECT name, dirty FROM `+qualifiedTable(schema, byNameMigrationsTable))
	if err != nil {
		return nil, errors.Wrap(err, "failed to load applied migrations")
	}
	defer rows.Close()
	applied := make(map[string]bool)
	for rows.Next() {
		var (
			name  string
			dirty bool
		)
		if err := rows.Scan(&name, &dirty); err != nil {
			return nil, errors.Wrap(err, "failed to load applied migrations")
		}
		applied[name] = dirty
	}
	return applied, errors.Wrap(rows.Err(), "failed to load applied migrations")
}

func loadLegacy(ctx context.Context, q queryer, schema string) (map[uint64]bool, error) {
	exists, err := tableExists(ctx, q, schema, legacyMigrationsTable)
	if err != nil || !exists {
		return map[uint64]bool{}, err
	}
	rows, err := q.QueryContext(ctx, `SELECT version, dirty FROM `+qualifiedTable(schema, legacyMigrationsTable))
	if err != nil {
		return nil, errors.Wrap(err, "failed to load schema_migrations")
	}
	defer rows.Close()
	legacy := make(map[uint64]bool)
	for rows.Next() {
		var (
			version int64
			dirty   bool
		)
		if err := rows.Scan(&version, &dirty); err != nil {
			return nil, errors.Wrap(err, "failed to load schema_migrations")
		}
		if version >= 0 {
			legacy[uint64(version)] = dirty
		}
	}
	return legacy, errors.Wrap(rows.Err(), "failed to load schema_migrations")
}

// loadAppliedByName returns the name-based state of the database without
// modifying it. If the database hasn't been upgraded to name-based tracking yet,
// the state is computed from schema_migrations the same way the upgrade would.
func loadAppliedByName(ctx context.Context, conn *sql.Conn, schema string, migs []*meta.DBMigration, keys []string, allowNonSeq bool) (map[string]bool, error) {
	exists, err := tableExists(ctx, conn, schema, byNameMigrationsTable)
	if err != nil {
		return nil, err
	} else if exists {
		return loadByName(ctx, conn, schema)
	}
	legacy, err := loadLegacy(ctx, conn, schema)
	if err != nil {
		return nil, err
	}
	return backfillFromLegacy(migs, keys, legacy, allowNonSeq), nil
}

// ensureByNameTable creates the name-based tracking table if necessary, and
// backfills it from schema_migrations when upgrading an existing database.
func ensureByNameTable(ctx context.Context, conn *sql.Conn, schema string, migs []*meta.DBMigration, keys []string, allowNonSeq bool) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	exists, err := tableExists(ctx, tx, schema, byNameMigrationsTable)
	if err != nil {
		return err
	} else if exists {
		return nil
	}

	_, err = tx.ExecContext(ctx, `CREATE TABLE `+qualifiedTable(schema, byNameMigrationsTable)+` (
		name       TEXT PRIMARY KEY,
		number     BIGINT NOT NULL,
		filename   TEXT NOT NULL,
		dirty      BOOLEAN NOT NULL DEFAULT false,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", byNameMigrationsTable)
	}

	legacy, err := loadLegacy(ctx, tx, schema)
	if err != nil {
		return err
	}
	backfill := backfillFromLegacy(migs, keys, legacy, allowNonSeq)
	for i, m := range migs {
		dirty, ok := backfill[keys[i]]
		if !ok {
			continue
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO `+qualifiedTable(schema, byNameMigrationsTable)+
			` (name, number, filename, dirty) VALUES ($1, $2, $3, $4)`,
			keys[i], int64(m.Number), m.Filename, dirty)
		if err != nil {
			return errors.Wrap(err, "failed to backfill applied migrations")
		}
	}
	return errors.Wrap(tx.Commit(), "failed to commit backfill")
}

// markStatement returns an SQL statement that records the migration as applied
// (or dirty) in the name-based table. It inlines all values so it can be appended
// to the migration and executed with the simple query protocol.
func markStatement(schema, key string, m *meta.DBMigration, dirty bool) string {
	return fmt.Sprintf(`INSERT INTO %s (name, number, filename, dirty, applied_at) VALUES (%s, %d, %s, %t, now())
ON CONFLICT (name) DO UPDATE SET number = EXCLUDED.number, filename = EXCLUDED.filename, dirty = EXCLUDED.dirty, applied_at = EXCLUDED.applied_at;`,
		qualifiedTable(schema, byNameMigrationsTable), pq.QuoteLiteral(key), m.Number, pq.QuoteLiteral(m.Filename), dirty)
}

// syncLegacyTable rewrites schema_migrations from the name-based state so an
// upstream Encore CLI sees a consistent state if the developer switches back.
func syncLegacyTable(ctx context.Context, conn *sql.Conn, schema string, migs []*meta.DBMigration, keys []string, applied map[string]bool, allowNonSeq bool) error {
	numbers := appliedNumbers(migs, keys, applied)
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	table := qualifiedTable(schema, legacyMigrationsTable)
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+table+` (version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL)`); err != nil {
		return errors.Wrap(err, "failed to create schema_migrations")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+table); err != nil {
		return errors.Wrap(err, "failed to reset schema_migrations")
	}
	insert := `INSERT INTO ` + table + ` (version, dirty) VALUES ($1, $2)`
	if allowNonSeq {
		for _, m := range migs {
			if dirty, ok := numbers[m.Number]; ok {
				if _, err := tx.ExecContext(ctx, insert, int64(m.Number), dirty); err != nil {
					return errors.Wrap(err, "failed to update schema_migrations")
				}
			}
		}
	} else if len(numbers) > 0 {
		// golang-migrate's sequential mode stores only the latest version.
		var latest uint64
		for v := range numbers {
			latest = max(latest, v)
		}
		if _, err := tx.ExecContext(ctx, insert, int64(latest), numbers[latest]); err != nil {
			return errors.Wrap(err, "failed to update schema_migrations")
		}
	}
	return errors.Wrap(tx.Commit(), "failed to commit schema_migrations")
}

// runMigrationsByName applies all pending migrations, tracking them by name.
// It returns the number of migrations applied.
func runMigrationsByName(ctx context.Context, dbName string, allowNonSeq bool, conn *sql.Conn, mdSrc *MetadataSource) (int, error) {
	if mdSrc.err != nil {
		return 0, mdSrc.err
	}
	migs := sortedMigrations(mdSrc.migrations)
	keys := migrationKeys(migs)

	schema, err := currentSchema(ctx, conn)
	if err != nil {
		return 0, err
	}

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, int64(byNameAdvisoryLockID)); err != nil {
		return 0, errors.Wrap(err, "failed to acquire migration lock")
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, int64(byNameAdvisoryLockID))
	}()

	if err := ensureByNameTable(ctx, conn, schema, migs, keys, allowNonSeq); err != nil {
		return 0, err
	}
	applied, err := loadByName(ctx, conn, schema)
	if err != nil {
		return 0, err
	}

	// Record the current number and filename of renumbered migrations
	// that have already been applied, for visibility.
	if _, err := conn.ExecContext(ctx, `UPDATE `+qualifiedTable(schema, byNameMigrationsTable)+` t
		SET number = c.number, filename = c.filename
		FROM unnest($1::text[], $2::bigint[], $3::text[]) AS c(name, number, filename)
		WHERE t.name = c.name AND (t.number <> c.number OR t.filename <> c.filename)`,
		keys, migrationNumbers(migs), migrationFilenames(migs)); err != nil {
		return 0, errors.Wrap(err, "failed to update renumbered migrations")
	}

	pending := pendingByName(keys, applied)
	count := 0
	for _, idx := range pending {
		m, key := migs[idx], keys[idx]

		// Mark the migration dirty first, so a migration that can't run in a
		// transaction and fails halfway is retried on the next run.
		if _, err := conn.ExecContext(ctx, markStatement(schema, key, m, true)); err != nil {
			return count, errors.Wrapf(err, "failed to mark migration %s as dirty", m.Filename)
		}
		applied[key] = true

		body, err := readMigration(mdSrc, m)
		if err != nil {
			return count, errors.Wrapf(err, "failed to read migration %s", m.Filename)
		}
		// Run the migration and mark it as applied in the same statement, so the
		// two happen atomically (Postgres runs a multi-statement simple query in
		// an implicit transaction unless the migration manages its own).
		query := body + "\n;\n" + markStatement(schema, key, m, false)
		if _, err := conn.ExecContext(ctx, query); err != nil {
			return count, errors.Wrapf(err, "migration %s failed for database %s", m.Filename, dbName)
		}
		applied[key] = false
		count++
	}

	if err := syncLegacyTable(ctx, conn, schema, migs, keys, applied, allowNonSeq); err != nil {
		return count, err
	}
	return count, nil
}

func readMigration(src *MetadataSource, m *meta.DBMigration) (string, error) {
	r, err := src.Read(m)
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(r)
	return string(data), err
}

// listAppliedByName returns the applied migrations of dbMeta, mapped onto the
// current migration numbers (number -> dirty).
func listAppliedByName(ctx context.Context, conn *sql.Conn, dbMeta *meta.SQLDatabase) (map[uint64]bool, error) {
	schema, err := currentSchema(ctx, conn)
	if err != nil {
		return nil, err
	}
	migs := sortedMigrations(dbMeta.Migrations)
	keys := migrationKeys(migs)
	applied, err := loadAppliedByName(ctx, conn, schema, migs, keys, dbMeta.AllowNonSequentialMigrations)
	if err != nil {
		return nil, err
	}
	return appliedNumbers(migs, keys, applied), nil
}

func migrationNumbers(migs []*meta.DBMigration) []int64 {
	out := make([]int64, len(migs))
	for i, m := range migs {
		out[i] = int64(m.Number)
	}
	return out
}

func migrationFilenames(migs []*meta.DBMigration) []string {
	out := make([]string, len(migs))
	for i, m := range migs {
		out[i] = m.Filename
	}
	return out
}
