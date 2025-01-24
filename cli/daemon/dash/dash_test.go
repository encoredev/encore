package dash

import (
	"reflect"
	"testing"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

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
