package dash

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"encr.dev/cli/daemon/apps"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

func TestAppDisplayNamesDistinguishesWorktrees(t *testing.T) {
	root := t.TempDir()
	primaryRoot := filepath.Join(root, "project", "apps", "backend")
	worktreeRoot := filepath.Join(root, "worktrees", "DRE-103", "apps", "backend")
	if err := os.MkdirAll(filepath.Join(root, "project", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "worktrees", "DRE-103", ".git"), []byte("gitdir: /tmp/repo/.git/worktrees/DRE-103\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	primary := apps.NewInstance(primaryRoot, "primary-local", "linked-app")
	worktree := apps.NewInstance(worktreeRoot, "worktree-local", "linked-app")
	other := apps.NewInstance(primaryRoot, "other-local", "other-app")
	got := appDisplayNames([]*apps.Instance{primary, worktree, other})

	if got[primary] != "linked-app (primary)" {
		t.Errorf("primary display name = %q, want %q", got[primary], "linked-app (primary)")
	}
	if got[worktree] != "linked-app (DRE-103)" {
		t.Errorf("worktree display name = %q, want %q", got[worktree], "linked-app (DRE-103)")
	}
	if got[other] != "other-app" {
		t.Errorf("unique display name = %q, want %q", got[other], "other-app")
	}
}

func TestAppDisplayNamesFallsBackToRoot(t *testing.T) {
	root := t.TempDir()
	firstRoot := filepath.Join(root, "first", "project", "apps", "backend")
	secondRoot := filepath.Join(root, "second", "project", "apps", "backend")
	for _, appRoot := range []string{firstRoot, secondRoot} {
		if err := os.MkdirAll(filepath.Join(appRoot, "..", "..", ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	first := apps.NewInstance(firstRoot, "first-local", "linked-app")
	second := apps.NewInstance(secondRoot, "second-local", "linked-app")
	got := appDisplayNames([]*apps.Instance{first, second})

	if got[first] != "linked-app ("+firstRoot+")" {
		t.Errorf("first display name = %q, want root qualifier", got[first])
	}
	if got[second] != "linked-app ("+secondRoot+")" {
		t.Errorf("second display name = %q, want root qualifier", got[second])
	}
}

func TestBuildMigrationHistory(t *testing.T) {
	tests := []struct {
		name            string
		dbMeta          *meta.SQLDatabase
		appliedVersions map[uint64]bool
		want            dbMigrationHistory
	}{
		{
			name: "sequential migrations all applied cleanly",
			dbMeta: &meta.SQLDatabase{
				Name: "test-db",
				Migrations: []*meta.DBMigration{
					{Number: 1, Filename: "001.sql", Description: "first"},
					{Number: 2, Filename: "002.sql", Description: "second"},
					{Number: 3, Filename: "003.sql", Description: "third"},
				},
				AllowNonSequentialMigrations: false,
			},
			appliedVersions: map[uint64]bool{
				1: false, // clean
				2: false, // clean
				3: false, // clean
			},
			want: dbMigrationHistory{
				DatabaseName: "test-db",
				Migrations: []dbMigration{
					{Number: 3, Filename: "003.sql", Description: "third", Applied: true},
					{Number: 2, Filename: "002.sql", Description: "second", Applied: true},
					{Number: 1, Filename: "001.sql", Description: "first", Applied: true},
				},
			},
		},
		{
			name: "sequential migrations with dirty migration",
			dbMeta: &meta.SQLDatabase{
				Name: "test-db",
				Migrations: []*meta.DBMigration{
					{Number: 1, Filename: "001.sql", Description: "first"},
					{Number: 2, Filename: "002.sql", Description: "second"},
					{Number: 3, Filename: "003.sql", Description: "third"},
				},
				AllowNonSequentialMigrations: false,
			},
			appliedVersions: map[uint64]bool{
				1: false, // clean
				2: true,  // dirty
			},
			want: dbMigrationHistory{
				DatabaseName: "test-db",
				Migrations: []dbMigration{
					{Number: 3, Filename: "003.sql", Description: "third", Applied: false},
					{Number: 2, Filename: "002.sql", Description: "second", Applied: false},
					{Number: 1, Filename: "001.sql", Description: "first", Applied: true},
				},
			},
		},
		{
			name: "sequential migrations partially applied",
			dbMeta: &meta.SQLDatabase{
				Name: "test-db",
				Migrations: []*meta.DBMigration{
					{Number: 1, Filename: "001.sql", Description: "first"},
					{Number: 2, Filename: "002.sql", Description: "second"},
					{Number: 3, Filename: "003.sql", Description: "third"},
				},
				AllowNonSequentialMigrations: false,
			},
			appliedVersions: map[uint64]bool{
				1: false, // clean
				2: false, // clean
			},
			want: dbMigrationHistory{
				DatabaseName: "test-db",
				Migrations: []dbMigration{
					{Number: 3, Filename: "003.sql", Description: "third", Applied: false},
					{Number: 2, Filename: "002.sql", Description: "second", Applied: true},
					{Number: 1, Filename: "001.sql", Description: "first", Applied: true},
				},
			},
		},
		{
			name: "non-sequential migrations with mix of clean and dirty",
			dbMeta: &meta.SQLDatabase{
				Name: "test-db",
				Migrations: []*meta.DBMigration{
					{Number: 1, Filename: "001.sql", Description: "first"},
					{Number: 2, Filename: "002.sql", Description: "second"},
					{Number: 3, Filename: "003.sql", Description: "third"},
				},
				AllowNonSequentialMigrations: true,
			},
			appliedVersions: map[uint64]bool{
				1: false, // clean
				2: true,  // dirty
				3: false, // clean
			},
			want: dbMigrationHistory{
				DatabaseName: "test-db",
				Migrations: []dbMigration{
					{Number: 3, Filename: "003.sql", Description: "third", Applied: true},
					{Number: 2, Filename: "002.sql", Description: "second", Applied: false},
					{Number: 1, Filename: "001.sql", Description: "first", Applied: true},
				},
			},
		},
		{
			name: "empty migrations list",
			dbMeta: &meta.SQLDatabase{
				Name:                         "test-db",
				Migrations:                   []*meta.DBMigration{},
				AllowNonSequentialMigrations: false,
			},
			appliedVersions: map[uint64]bool{},
			want: dbMigrationHistory{
				DatabaseName: "test-db",
				Migrations:   []dbMigration{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMigrationHistory(tt.dbMeta, tt.appliedVersions)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildMigrationHistory() = %v, want %v", got, tt.want)
			}
		})
	}
}
