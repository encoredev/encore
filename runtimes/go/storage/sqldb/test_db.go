package sqldb

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/rs/xid"
)

//publicapigen:drop
func (mgr *Manager) NewTestDatabase(ctx context.Context, name string) (*Database, error) {
	db := mgr.GetDB(name)
	if db.noopDB {
		return nil, fmt.Errorf("et: unknown database name: %q", name)
	}

	dbName := db.origName + "_" + xid.New().String()
	templateName := db.origName + "_template"
	if _, err := db.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s",
		pgx.Identifier{dbName}.Sanitize(),
		pgx.Identifier{templateName}.Sanitize(),
	)); err != nil {
		return nil, err
	}

	clone := &Database{
		name:     dbName,
		origName: db.origName,
		mgr:      mgr,
	}

	mgr.ts.AddEndCallback(func(t *testing.T) {
		// Shut down the connection pools and attempt to drop the database.
		clone.shutdown()
		_, _ = db.Exec(ctx, fmt.Sprintf("DROP DATABASE %s WITH (FORCE)",
			pgx.Identifier{dbName}.Sanitize()))
	})
	return clone, nil
}
