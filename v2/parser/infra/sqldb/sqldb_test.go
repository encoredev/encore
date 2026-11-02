package sqldb

import (
	"testing"
	"time"

	"encr.dev/v2/parser/resource/resourcetest"
)

func TestParseDatabase(t *testing.T) {
	tests := []resourcetest.Case[*Database]{
		{
			Name: "constructor",
			Code: `
var x = sqldb.NewDatabase("name", sqldb.DatabaseConfig{
	Migrations: "some/migration/path",
})
-- some/migration/path/foo.txt --
`,
			Want: &Database{
				Name:         "name",
				MigrationDir: "some/migration/path",
			},
		},
		{
			Name: "migration_file",
			Code: `
var x = sqldb.NewDatabase("name", sqldb.DatabaseConfig{
	Migrations: "some/migration/path",
})
-- some/migration/path/1_foo.up.sql --
CREATE TABLE foo (id int);
`,
			Want: &Database{
				Name:         "name",
				MigrationDir: "some/migration/path",
				Migrations: []MigrationFile{{
					Filename:    "1_foo.up.sql",
					Number:      1,
					Description: "foo",
				}},
			},
		},
		{
			Name: "abs_path",
			Code: `
var x = sqldb.NewDatabase("name", sqldb.DatabaseConfig{
	Migrations: "/path",
})
`,
			WantErrs: []string{`.*The migration path must be a relative path.*`},
		},
		{
			Name: "non_local_path",
			Code: `
var x = sqldb.NewDatabase("name", sqldb.DatabaseConfig{
	Migrations: "../path",
})
`,
			WantErrs: []string{`.*The migration path must be a relative path.*`},
		},
		{
			Name:    "conn_lifetime",
			Imports: []string{"time"},
			Code: `
var x = sqldb.NewDatabase("name", sqldb.DatabaseConfig{
	Migrations:            "some/migration/path",
	MaxConnLifetime:       30 * time.Minute,
	MaxConnLifetimeJitter: 10 * time.Minute,
})
-- some/migration/path/foo.txt --
`,
			Want: &Database{
				Name:                  "name",
				MigrationDir:          "some/migration/path",
				MaxConnLifetime:       30 * time.Minute,
				MaxConnLifetimeJitter: 10 * time.Minute,
			},
		},
		{
			Name:    "conn_lifetime_negative",
			Imports: []string{"time"},
			Code: `
var x = sqldb.NewDatabase("name", sqldb.DatabaseConfig{
	Migrations:            "some/migration/path",
	MaxConnLifetime:       -time.Minute,
	MaxConnLifetimeJitter: -time.Second,
})
-- some/migration/path/foo.txt --
`,
			WantErrs: []string{
				`.*The max connection lifetime must not be negative.*`,
				`.*The max connection lifetime jitter must not be negative.*`,
			},
		},
		{
			Name:    "conn_lifetime_not_constant",
			Imports: []string{"time"},
			Code: `
var lifetime = time.Minute

var x = sqldb.NewDatabase("name", sqldb.DatabaseConfig{
	Migrations:      "some/migration/path",
	MaxConnLifetime: lifetime,
})
-- some/migration/path/foo.txt --
`,
			WantErrs: []string{"(?s).*Field `MaxConnLifetime` must be a constant literal.*"},
		},
	}

	resourcetest.Run(t, DatabaseParser, tests)
}
