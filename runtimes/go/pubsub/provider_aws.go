//go:build !encore_no_aws

package pubsub

import "encore.dev/pubsub/internal/aws"

func init() {
	registerProvider(func(mgr *Manager) provider {
		return aws.NewManager(mgr.ctxs)
	})
}
