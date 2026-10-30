package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/jackc/pgx/v5/stdlib"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

func setTrackByName(t *testing.T, enabled bool) {
	t.Helper()
	orig := trackMigrationsByName
	trackMigrationsByName = func() bool { return enabled }
	t.Cleanup(func() { trackMigrationsByName = orig })
}

func migs(filenames ...string) []*meta.DBMigration {
	out := make([]*meta.DBMigration, len(filenames))
	for i, fn := range filenames {
		var num uint64
		_, _ = fmt.Sscanf(fn, "%d", &num)
		out[i] = &meta.DBMigration{Filename: fn, Number: num}
	}
	return out
}

func TestMigrationName(t *testing.T) {
	c := qt.New(t)
	for fn, want := range map[string]string{
		"1002_add_users.up.sql": "add_users",
		"0003_brave_hulk.sql":   "brave_hulk",
		"7.up.sql":              "7.up.sql",
		"12_x.y.up.sql":         "x.y",
	} {
		c.Assert(migrationName(&meta.DBMigration{Filename: fn}), qt.Equals, want, qt.Commentf("%s", fn))
	}
}

func TestMigrationKeysDuplicates(t *testing.T) {
	c := qt.New(t)
	keys := migrationKeys(migs("1_add_index.up.sql", "2_users.up.sql", "3_add_index.up.sql"))
	c.Assert(keys, qt.DeepEquals, []string{"add_index", "users", "add_index#2"})
}

func TestPendingByNameRenumber(t *testing.T) {
	c := qt.New(t)
	applied := map[string]bool{"alice": false, "bob": false, "carol": false}
	m := migs("1000_alice.up.sql", "1001_bob.up.sql", "1002_joe.up.sql", "1003_carol.up.sql")
	keys := migrationKeys(m)
	var got []string
	for _, idx := range pendingByName(keys, applied) {
		got = append(got, m[idx].Filename)
	}
	c.Assert(got, qt.DeepEquals, []string{"1002_joe.up.sql"})

	// A dirty migration is re-applied.
	applied["bob"] = true
	got = nil
	for _, idx := range pendingByName(keys, applied) {
		got = append(got, m[idx].Filename)
	}
	c.Assert(got, qt.DeepEquals, []string{"1001_bob.up.sql", "1002_joe.up.sql"})
}

func TestBackfillFromLegacy(t *testing.T) {
	c := qt.New(t)
	m := migs("1000_alice.up.sql", "1001_bob.up.sql", "1002_carol.up.sql", "1003_dave.up.sql")
	keys := migrationKeys(m)
	testCases := map[string]struct {
		nonSeq bool
		legacy map[uint64]bool
		want   map[string]bool
	}{
		"empty":       {legacy: map[uint64]bool{}, want: map[string]bool{}},
		"seq":         {legacy: map[uint64]bool{1002: false}, want: map[string]bool{"alice": false, "bob": false, "carol": false}},
		"seq_dirty":   {legacy: map[uint64]bool{1002: true}, want: map[string]bool{"alice": false, "bob": false, "carol": true}},
		"seq_deleted": {legacy: map[uint64]bool{1005: false}, want: map[string]bool{"alice": false, "bob": false, "carol": false, "dave": false}},
		"nonseq":      {nonSeq: true, legacy: map[uint64]bool{1000: false, 1002: true}, want: map[string]bool{"alice": false, "carol": true}},
	}
	for name, tc := range testCases {
		c.Run(name, func(c *qt.C) {
			c.Assert(backfillFromLegacy(m, keys, tc.legacy, tc.nonSeq), qt.DeepEquals, tc.want)
		})
	}
}

// The tests below need a throwaway Postgres server. Set
// ENCORE_MIGRATIONS_TEST_PG to a connection URI to the server's "postgres"
// database; each test creates and drops its own database.

func testDB(t *testing.T) (*sql.Conn, func(string, ...any) *sql.Rows) {
	t.Helper()
	uri := os.Getenv("ENCORE_MIGRATIONS_TEST_PG")
	if uri == "" {
		t.Skip("ENCORE_MIGRATIONS_TEST_PG not set")
	}
	ctx := context.Background()
	admin, err := sql.Open("pgx", uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	name := "migtest_" + strings.ToLower(strings.NewReplacer("/", "_", "-", "_").Replace(t.Name()))
	_, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name)
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}

	u, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	pool, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pool.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		_ = pool.Close()
		_, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	query := func(q string, args ...any) *sql.Rows {
		rows, err := conn.QueryContext(ctx, q, args...)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	return conn, query
}

// migrationDir writes the given files (filename -> SQL) to a fresh directory and
// returns a MetadataSource for them.
func migrationDir(t *testing.T, files map[string]string) *MetadataSource {
	t.Helper()
	dir := t.TempDir()
	var names []string
	for fn, body := range files {
		if err := os.WriteFile(filepath.Join(dir, fn), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		names = append(names, fn)
	}
	return NewMetadataSource(NewOsMigrationReader(dir), sortedMigrations(migs(names...)))
}

func scanStrings(t *testing.T, rows *sql.Rows) []string {
	t.Helper()
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func legacyRows(t *testing.T, query func(string, ...any) *sql.Rows) []string {
	return scanStrings(t, query(`SELECT version::text || ':' || dirty::text FROM schema_migrations ORDER BY version`))
}

func TestRunMigrationsByNameRenumber(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	conn, query := testDB(t)

	// The developer is on their branch, which added 1002_carol.
	src := migrationDir(t, map[string]string{
		"1000_alice.up.sql": "CREATE TABLE alice (id int);",
		"1001_bob.up.sql":   "CREATE TABLE bob (id int);",
		"1002_carol.up.sql": "CREATE TABLE carol (id int);",
	})
	n, err := runMigrationsByName(ctx, "test", false, conn, src)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 3)
	c.Assert(legacyRows(t, query), qt.DeepEquals, []string{"1002:false"})

	// They rebase onto main, which has 1002_joe, and carol is renumbered to 1003.
	src = migrationDir(t, map[string]string{
		"1000_alice.up.sql": "CREATE TABLE alice (id int);",
		"1001_bob.up.sql":   "CREATE TABLE bob (id int);",
		"1002_joe.up.sql":   "CREATE TABLE joe (id int);",
		"1003_carol.up.sql": "CREATE TABLE carol (id int);", // would fail if re-applied
	})
	n, err = runMigrationsByName(ctx, "test", false, conn, src)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)
	c.Assert(scanStrings(t, query(`SELECT tablename::text FROM pg_tables WHERE schemaname = 'public' AND tablename IN ('alice','bob','carol','joe') ORDER BY 1`)),
		qt.DeepEquals, []string{"alice", "bob", "carol", "joe"})
	c.Assert(scanStrings(t, query(`SELECT name || ':' || number::text || ':' || dirty::text FROM encore_schema_migrations ORDER BY number`)),
		qt.DeepEquals, []string{"alice:1000:false", "bob:1001:false", "joe:1002:false", "carol:1003:false"})
	c.Assert(legacyRows(t, query), qt.DeepEquals, []string{"1003:false"})

	// Running again is a no-op.
	n, err = runMigrationsByName(ctx, "test", false, conn, src)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)

	// listAppliedByName reports everything as applied in the current numbering.
	applied, err := listAppliedByName(ctx, conn, &meta.SQLDatabase{Migrations: src.migrations})
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.DeepEquals, map[uint64]bool{1000: false, 1001: false, 1002: false, 1003: false})
}

func TestRunMigrationsByNameUpgradesLegacyTable(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	conn, query := testDB(t)

	// A database migrated by upstream Encore up to 1001, golang-migrate style.
	_, err := conn.ExecContext(ctx, `
		CREATE TABLE alice (id int);
		CREATE TABLE bob (id int);
		CREATE TABLE schema_migrations (version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL);
		INSERT INTO schema_migrations VALUES (1001, false);`)
	c.Assert(err, qt.IsNil)

	src := migrationDir(t, map[string]string{
		"1000_alice.up.sql": "CREATE TABLE alice (id int);",
		"1001_bob.up.sql":   "CREATE TABLE bob (id int);",
		"1002_carol.up.sql": "CREATE TABLE carol (id int);",
	})

	// Before the upgrade, the read-only view is computed from schema_migrations.
	applied, err := listAppliedByName(ctx, conn, &meta.SQLDatabase{Migrations: src.migrations})
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.DeepEquals, map[uint64]bool{1000: false, 1001: false})

	n, err := runMigrationsByName(ctx, "test", false, conn, src)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)
	c.Assert(scanStrings(t, query(`SELECT name || ':' || dirty::text FROM encore_schema_migrations ORDER BY number`)),
		qt.DeepEquals, []string{"alice:false", "bob:false", "carol:false"})
	c.Assert(legacyRows(t, query), qt.DeepEquals, []string{"1002:false"})
}

func TestRunMigrationsByNameUpgradesDirtyLegacyTable(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	conn, _ := testDB(t)

	// 1001 failed under upstream Encore (rolled back, marked dirty).
	_, err := conn.ExecContext(ctx, `
		CREATE TABLE alice (id int);
		CREATE TABLE schema_migrations (version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL);
		INSERT INTO schema_migrations VALUES (1001, true);`)
	c.Assert(err, qt.IsNil)

	src := migrationDir(t, map[string]string{
		"1000_alice.up.sql": "CREATE TABLE alice (id int);",
		"1001_bob.up.sql":   "CREATE TABLE bob (id int);",
	})
	n, err := runMigrationsByName(ctx, "test", false, conn, src)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)
}

func TestRunMigrationsByNameFailureIsRetried(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	conn, query := testDB(t)

	src := migrationDir(t, map[string]string{
		"1_alice.up.sql": "CREATE TABLE alice (id int);",
		"2_bob.up.sql":   "CREATE TABLE bob (id int); SELECT * FROM does_not_exist;",
	})
	n, err := runMigrationsByName(ctx, "test", false, conn, src)
	c.Assert(err, qt.ErrorMatches, `(?s).*2_bob.up.sql.*`)
	c.Assert(n, qt.Equals, 1)
	// bob is recorded as dirty and its changes were rolled back.
	c.Assert(scanStrings(t, query(`SELECT name || ':' || dirty::text FROM encore_schema_migrations ORDER BY number`)),
		qt.DeepEquals, []string{"alice:false", "bob:true"})
	c.Assert(scanStrings(t, query(`SELECT tablename::text FROM pg_tables WHERE tablename = 'bob'`)), qt.HasLen, 0)

	// Fix the migration and run again.
	src = migrationDir(t, map[string]string{
		"1_alice.up.sql": "CREATE TABLE alice (id int);",
		"2_bob.up.sql":   "CREATE TABLE bob (id int);",
	})
	n, err = runMigrationsByName(ctx, "test", false, conn, src)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)
	c.Assert(legacyRows(t, query), qt.DeepEquals, []string{"2:false"})
}

func TestRunMigrationsByNameNonSequential(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	conn, query := testDB(t)

	_, err := conn.ExecContext(ctx, `
		CREATE TABLE alice (id int);
		CREATE TABLE carol (id int);
		CREATE TABLE schema_migrations (version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL);
		INSERT INTO schema_migrations VALUES (1, false), (3, false);`)
	c.Assert(err, qt.IsNil)

	src := migrationDir(t, map[string]string{
		"1_alice.up.sql": "CREATE TABLE alice (id int);",
		"2_bob.up.sql":   "CREATE TABLE bob (id int);",
		"3_carol.up.sql": "CREATE TABLE carol (id int);",
	})
	n, err := runMigrationsByName(ctx, "test", true, conn, src)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)
	c.Assert(legacyRows(t, query), qt.DeepEquals, []string{"1:false", "2:false", "3:false"})
}
