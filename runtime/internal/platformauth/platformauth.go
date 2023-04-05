// Package platformauth contains contexts for marking a request
// as being authenticated to come from the Encore platform.
//
// It's an internal package to ensure only the Encore runtime can
// mark requests as such.
package platformauth

import "context"

type ctxKey string

const platformAuthCtxKey ctxKey = "platformAuthCtxKey"

func WithEncorePlatformSealOfApproval(ctx context.Context) context.Context {
	return context.WithValue(ctx, platformAuthCtxKey, true)
}

// IsEncorePlatformRequest returns true if the given context originated from
// a request from the Encore Platform.
func IsEncorePlatformRequest(ctx context.Context) bool {
	value := ctx.Value(platformAuthCtxKey)
	if value == nil {
		return false
	}

	v, ok := value.(bool)
	return ok && v
}
