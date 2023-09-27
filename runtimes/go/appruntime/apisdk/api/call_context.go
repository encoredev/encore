//go:build encore_app

package api

import (
	"context"
)

func NewCallContext(ctx context.Context) CallContext {
	return Singleton.NewCallContext(ctx)
}
