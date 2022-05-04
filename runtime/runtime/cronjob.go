//go:build encore_internal

package runtime

import (
	"context"
)

type cronjobIdContextKeyType string

const cronjobIdContextKey cronjobIdContextKeyType = "cronjob-id"

func contextWithCronJobID(ctx context.Context, cronJobId string) context.Context {
	return context.WithValue(ctx, cronjobIdContextKey, cronJobId)
}

func CronJobIdFromContext(ctx context.Context) string {
	v, ok := ctx.Value(cronjobIdContextKey).(string)
	if !ok {
		return ""
	}

	return v
}
