package sqldb

import (
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

// Any change to a database's migrations must change its hash.
func TestHashMigrations(t *testing.T) {
	c := qt.New(t)
	write := func(c *qt.C, files map[string]string) (string, *meta.SQLDatabase) {
		root := c.TempDir()
		relPath := "migrations"
		c.Assert(os.Mkdir(filepath.Join(root, relPath), 0o755), qt.IsNil)
		dbMeta := &meta.SQLDatabase{MigrationRelPath: &relPath}
		for _, name := range []string{"1_a.up.sql", "2_b.up.sql", "2_c.up.sql"} {
			if body, ok := files[name]; ok {
				c.Assert(os.WriteFile(filepath.Join(root, relPath, name), []byte(body), 0o644), qt.IsNil)
				dbMeta.Migrations = append(dbMeta.Migrations, &meta.DBMigration{Filename: name})
			}
		}
		return root, dbMeta
	}
	hash := func(c *qt.C, files map[string]string) string {
		root, dbMeta := write(c, files)
		h, err := HashMigrations(root, dbMeta)
		c.Assert(err, qt.IsNil)
		return h
	}
	base := map[string]string{"1_a.up.sql": "CREATE TABLE a ();", "2_b.up.sql": "CREATE TABLE b ();"}

	c.Run("same migrations", func(c *qt.C) {
		c.Assert(hash(c, base), qt.Equals, hash(c, base))
	})
	c.Run("edited migration", func(c *qt.C) {
		c.Assert(hash(c, map[string]string{"1_a.up.sql": "CREATE TABLE a ();", "2_b.up.sql": "CREATE TABLE b (id int);"}), qt.Not(qt.Equals), hash(c, base))
	})
	c.Run("renamed migration", func(c *qt.C) {
		c.Assert(hash(c, map[string]string{"1_a.up.sql": "CREATE TABLE a ();", "2_c.up.sql": "CREATE TABLE b ();"}), qt.Not(qt.Equals), hash(c, base))
	})
	c.Run("added migration", func(c *qt.C) {
		c.Assert(hash(c, map[string]string{"1_a.up.sql": "CREATE TABLE a ();"}), qt.Not(qt.Equals), hash(c, base))
	})
	c.Run("no migrations", func(c *qt.C) {
		c.Assert(hash(c, nil), qt.Equals, "")
	})
	c.Run("missing file", func(c *qt.C) {
		root, dbMeta := write(c, base)
		c.Assert(os.Remove(filepath.Join(root, "migrations", "2_b.up.sql")), qt.IsNil)
		_, err := HashMigrations(root, dbMeta)
		c.Assert(err, qt.IsNotNil)
	})
}
