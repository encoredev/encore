package gcp

import (
	"fmt"

	"cloud.google.com/go/pubsub"
)

// getClient returns a singleton pubsub client for the given project or panics if it cannot be created.
func (mgr *Manager) getClient() *pubsub.Client {
	mgr.clientOnce.Do(func() {
		// Create a new client
		cl, err := pubsub.NewClient(mgr.ctxs.Connection, "")
		if err != nil {
			panic(fmt.Sprintf("failed to create pubsub client: %s", err))
		}
		mgr._client = cl
	})
	return mgr._client
}
