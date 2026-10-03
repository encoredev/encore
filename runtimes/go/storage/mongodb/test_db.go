package mongodb

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/xid"
)

//publicapigen:drop
func (mgr *Manager) NewTestDatabase(ctx context.Context, name string) (*Database, error) {
	db := mgr.GetDB(name)
	if db.noopDB {
		return nil, fmt.Errorf("et: unknown MongoDB database name: %q", name)
	}

	// MongoDB creates the database on first write, so there is
	// nothing to create here; it only needs a unique name.
	clone := &Database{
		name:           db.name,
		mgr:            mgr,
		dbNameOverride: db.name + "_" + xid.New().String(),
	}

	mgr.ts.AddEndCallback(func(t *testing.T) {
		// Drop the database if it was used, and close the connections.
		if clone.db != nil {
			if err := clone.db.Drop(context.Background()); err != nil {
				t.Logf("failed to clean up test MongoDB database %s: %v", clone.dbNameOverride, err)
			}
		}
		clone.shutdown(context.Background())
	})
	return clone, nil
}
