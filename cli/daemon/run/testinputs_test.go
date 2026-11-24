package run

import (
	"os"
	"path/filepath"
	"testing"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

func TestTestInputsHash(t *testing.T) {
	// setup writes one migration file and returns inputs with two databases that use it.
	setup := func(t *testing.T) (testInputsParams, string) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "migrations"), 0o755); err != nil {
			t.Fatal(err)
		}
		migration := filepath.Join(root, "migrations", "1_create.up.sql")
		if err := os.WriteFile(migration, []byte("CREATE TABLE a (id INT);"), 0o644); err != nil {
			t.Fatal(err)
		}
		relPath := "migrations"
		migrations := []*meta.DBMigration{{Filename: "1_create.up.sql", Number: 1}}
		return testInputsParams{
			AppRoot:        root,
			AppFile:        []byte(`{"id": "app"}`),
			Secrets:        map[string]string{"Key": "a"},
			ServiceConfigs: map[string]string{"svc": "{limit: 1}"},
			Databases: []*meta.SQLDatabase{
				{Name: "db", MigrationRelPath: &relPath, Migrations: migrations},
				{Name: "other", MigrationRelPath: &relPath, Migrations: migrations},
			},
		}, migration
	}
	hash := func(t *testing.T, p testInputsParams) string {
		t.Helper()
		h, err := testInputsHash(p)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}

	t.Run("same inputs", func(t *testing.T) {
		p, _ := setup(t)
		if got, want := hash(t, p), hash(t, p); got != want {
			t.Errorf("hash = %s, want %s", got, want)
		}
	})

	t.Run("same databases in another order", func(t *testing.T) {
		p, _ := setup(t)
		want := hash(t, p)
		p.Databases = []*meta.SQLDatabase{p.Databases[1], p.Databases[0]}
		if got := hash(t, p); got != want {
			t.Errorf("hash = %s, want %s", got, want)
		}
	})

	changes := []struct {
		name   string
		change func(p *testInputsParams, migration string)
	}{
		{name: "app file", change: func(p *testInputsParams, _ string) { p.AppFile = []byte(`{"id": "app", "log_level": "debug"}`) }},
		{name: "secret value", change: func(p *testInputsParams, _ string) { p.Secrets = map[string]string{"Key": "b"} }},
		{name: "secret added", change: func(p *testInputsParams, _ string) { p.Secrets = map[string]string{"Key": "a", "Other": ""} }},
		{name: "secret removed", change: func(p *testInputsParams, _ string) { p.Secrets = nil }},
		{name: "service config", change: func(p *testInputsParams, _ string) { p.ServiceConfigs = map[string]string{"svc": "{limit: 2}"} }},
		{name: "migration", change: func(_ *testInputsParams, migration string) {
			if err := os.WriteFile(migration, []byte("CREATE TABLE a (id BIGINT);"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range changes {
		t.Run(tt.name+" changed", func(t *testing.T) {
			p, migration := setup(t)
			want := hash(t, p)
			tt.change(&p, migration)
			if got := hash(t, p); got == want {
				t.Errorf("hash unchanged after the %s changed", tt.name)
			}
		})
	}

	t.Run("migration missing", func(t *testing.T) {
		p, migration := setup(t)
		if err := os.Remove(migration); err != nil {
			t.Fatal(err)
		}
		if _, err := testInputsHash(p); err == nil {
			t.Error("no error for a missing migration file")
		}
	})
}
