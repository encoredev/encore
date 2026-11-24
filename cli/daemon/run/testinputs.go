package run

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"

	"github.com/cockroachdb/errors"

	"encr.dev/cli/daemon/sqldb"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// testInputsParams are what a Go test gets from the daemon and not from its compiled binary.
type testInputsParams struct {
	AppRoot string
	// The encore.app file, whose settings such as log_level reach tests only through the runtime config.
	AppFile []byte
	Secrets map[string]string
	// Service name to its CUE config, as the test receives it.
	ServiceConfigs map[string]string
	Databases      []*meta.SQLDatabase
}

// testInputsHash hashes p, so a change to any of it makes go test run the tests again.
func testInputsHash(p testInputsParams) (string, error) {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "appfile\x00%d\x00%s\x00", len(p.AppFile), p.AppFile)
	for _, k := range slices.Sorted(maps.Keys(p.Secrets)) {
		_, _ = fmt.Fprintf(h, "secret\x00%d\x00%s\x00%d\x00%s\x00", len(k), k, len(p.Secrets[k]), p.Secrets[k])
	}
	for _, k := range slices.Sorted(maps.Keys(p.ServiceConfigs)) {
		_, _ = fmt.Fprintf(h, "config\x00%d\x00%s\x00%d\x00%s\x00", len(k), k, len(p.ServiceConfigs[k]), p.ServiceConfigs[k])
	}
	// Two parses of unchanged code can list the databases in a different order.
	byName := func(a, b *meta.SQLDatabase) int { return cmp.Compare(a.Name, b.Name) }
	for _, db := range slices.SortedFunc(slices.Values(p.Databases), byName) {
		migrationsHash, err := sqldb.HashMigrations(p.AppRoot, db)
		if err != nil {
			return "", errors.Wrapf(err, "hash migrations of database %s", db.Name)
		}
		_, _ = fmt.Fprintf(h, "database\x00%s\x00%s\x00", db.Name, migrationsHash)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
