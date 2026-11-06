package sqldb

import (
	"testing"

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
	}

	resourcetest.Run(t, DatabaseParser, tests)
}
