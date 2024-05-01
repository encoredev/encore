package et

import (
	"context"

	"encore.dev/storage/sqldb"
)

func (mgr *Manager) NewTestDatabase(ctx context.Context, name string) (*sqldb.Database, error) {
	return mgr.db.NewTestDatabase(ctx, name)
}
