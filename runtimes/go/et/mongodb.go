package et

import (
	"context"

	"encore.dev/storage/mongodb"
)

func (mgr *Manager) NewTestMongoDatabase(ctx context.Context, name string) (*mongodb.Database, error) {
	return mgr.mongodb.NewTestDatabase(ctx, name)
}
