package infra

import (
	"math"
	"strconv"
	"testing"

	"github.com/rs/zerolog"
)

func TestSQLDatabaseMaxConnections(t *testing.T) {
	rm := &ResourceManager{log: zerolog.Nop()}

	tests := []struct {
		name    string
		envVar  string
		numDBs  int
		wantVal int
	}{
		{name: "default budget, one db", numDBs: 1, wantVal: 96},
		{name: "default budget, three dbs", numDBs: 3, wantVal: 32},
		{name: "default budget, six dbs", numDBs: 6, wantVal: 16},
		{name: "default budget, zero dbs returns zero", numDBs: 0, wantVal: 0},
		{name: "default budget, negative dbs returns zero", numDBs: -1, wantVal: 0},
		{name: "default budget floors at 1 when many dbs", numDBs: 200, wantVal: 1},
		{name: "custom budget", envVar: "400", numDBs: 10, wantVal: 40},
		{name: "custom budget floors at 1", envVar: "2", numDBs: 10, wantVal: 1},
		{name: "invalid env var falls back to default", envVar: "not-a-number", numDBs: 4, wantVal: 24},
		{name: "zero env var falls back to default", envVar: "0", numDBs: 4, wantVal: 24},
		{name: "negative env var falls back to default", envVar: "-5", numDBs: 4, wantVal: 24},
		{name: "budget larger than int32 max clamps to int32 max", envVar: strconv.Itoa(math.MaxInt32 + 1000), numDBs: 1, wantVal: math.MaxInt32},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENCORE_SQLDB_POOL_BUDGET", tc.envVar)
			if got := rm.SQLDatabaseMaxConnections(tc.numDBs); got != tc.wantVal {
				t.Fatalf("SQLDatabaseMaxConnections(%d) with env=%q = %d, want %d",
					tc.numDBs, tc.envVar, got, tc.wantVal)
			}
		})
	}
}
