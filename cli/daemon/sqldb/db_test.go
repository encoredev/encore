package sqldb

import (
	"io/fs"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/golang-migrate/migrate/v4/source/file" // for running migrations from the filesystem

	meta "encr.dev/proto/encore/parser/meta/v1"
)

func TestFindClosestVersion(t *testing.T) {
	c := qt.New(t)
	testCases := map[string]struct {
		versions    []uint
		dirty       int
		expected    int
		expectedErr bool
	}{
		"first": {
			versions: []uint{1, 2, 3},
			dirty:    1,
			expected: -1,
		},
		"middle": {
			versions: []uint{1, 2, 3},
			dirty:    2,
			expected: 1,
		},
		"last": {
			versions: []uint{1, 2, 3},
			dirty:    3,
			expected: 2,
		},
		"deleted": {
			versions: []uint{1, 2, 4},
			dirty:    3,
			expected: 2,
		},
		"deleted_first": {
			versions: []uint{2, 3, 4},
			dirty:    1,
			expected: -1,
		},
		"empty": {
			dirty:       5,
			expectedErr: true,
		},
	}

	for name, tc := range testCases {
		c.Run(name, func(c *qt.C) {
			result, err := findClosestLowerVersion(func() (uint, error) {
				if len(tc.versions) == 0 {
					return 0, fs.ErrNotExist
				}
				return tc.versions[0], nil
			}, tc.dirty, func(version uint) (uint, error) {
				for _, v := range tc.versions {
					if v > version {
						return v, nil
					}
				}
				return 0, fs.ErrNotExist
			})
			if tc.expectedErr {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(result, qt.Equals, tc.expected)
			}
		})
	}
}

func TestPendingMigrations(t *testing.T) {
	c := qt.New(t)
	migrations := []*meta.DBMigration{
		{Filename: "1_a.up.sql", Number: 1},
		{Filename: "2_b.up.sql", Number: 2},
		{Filename: "3_c.up.sql", Number: 3},
	}
	testCases := map[string]struct {
		nonSeq     bool
		migrations []*meta.DBMigration // defaults to migrations
		applied    map[uint64]bool
		expected   []string
	}{
		// Drizzle numbers its first migration 0.
		"seq_zero_none_applied": {
			migrations: []*meta.DBMigration{{Filename: "0000_init.sql", Number: 0}},
			applied:    map[uint64]bool{},
			expected:   []string{"0000_init.sql"},
		},
		"seq_zero_applied": {
			migrations: []*meta.DBMigration{{Filename: "0000_init.sql", Number: 0}},
			applied:    map[uint64]bool{0: false},
			expected:   nil,
		},
		"seq_none_applied":    {applied: map[uint64]bool{}, expected: []string{"1_a.up.sql", "2_b.up.sql", "3_c.up.sql"}},
		"seq_all_applied":     {applied: map[uint64]bool{3: false}, expected: nil},
		"seq_new_migration":   {applied: map[uint64]bool{2: false}, expected: []string{"3_c.up.sql"}},
		"seq_dirty":           {applied: map[uint64]bool{2: true}, expected: []string{"2_b.up.sql", "3_c.up.sql"}},
		"nonseq_gap":          {nonSeq: true, applied: map[uint64]bool{1: false, 3: false}, expected: []string{"2_b.up.sql"}},
		"nonseq_dirty":        {nonSeq: true, applied: map[uint64]bool{1: false, 2: false, 3: true}, expected: []string{"3_c.up.sql"}},
		"nonseq_all_applied":  {nonSeq: true, applied: map[uint64]bool{1: false, 2: false, 3: false}, expected: nil},
		"nonseq_none_applied": {nonSeq: true, applied: map[uint64]bool{}, expected: []string{"1_a.up.sql", "2_b.up.sql", "3_c.up.sql"}},
	}
	// These cases cover upstream number-based tracking.
	setTrackByName(t, false)
	for name, tc := range testCases {
		c.Run(name, func(c *qt.C) {
			dbMeta := &meta.SQLDatabase{Migrations: migrations, AllowNonSequentialMigrations: tc.nonSeq}
			if tc.migrations != nil {
				dbMeta.Migrations = tc.migrations
			}
			var got []string
			for _, m := range PendingMigrations("", dbMeta, tc.applied) {
				got = append(got, m.Filename)
			}
			c.Assert(got, qt.DeepEquals, tc.expected)
		})
	}
}

func TestPendingMigrationsByName(t *testing.T) {
	c := qt.New(t)
	setTrackByName(t, true)
	// alice, bob and carol were applied while carol was 1002; then joe landed
	// on main as 1002 and carol was renumbered to 1003. The applied versions are
	// reported in the current numbering, so only joe is pending.
	dbMeta := &meta.SQLDatabase{Migrations: []*meta.DBMigration{
		{Filename: "1000_alice.up.sql", Number: 1000},
		{Filename: "1001_bob.up.sql", Number: 1001},
		{Filename: "1002_joe.up.sql", Number: 1002},
		{Filename: "1003_carol.up.sql", Number: 1003},
	}}
	applied := map[uint64]bool{1000: false, 1001: false, 1003: false}
	var got []string
	for _, m := range PendingMigrations("", dbMeta, applied) {
		got = append(got, m.Filename)
	}
	c.Assert(got, qt.DeepEquals, []string{"1002_joe.up.sql"})
}
