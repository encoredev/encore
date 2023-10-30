package gcp

import (
	"fmt"

	"cloud.google.com/go/pubsub"
)

// getClient returns a singleton pubsub client for the given project or panics if it cannot be created.
func (mgr *Manager) getClientForProject(projectID string) *pubsub.Client {
	mgr.clientsMu.Lock()
	defer mgr.clientsMu.Unlock()

	client, ok := mgr.clients[projectID]
	if !ok {
		// Create a new client
		cl, err := pubsub.NewClient(mgr.ctxs.Connection, projectID)
		if err != nil {
			panic(fmt.Sprintf("failed to create pubsub client: %s", err))
		}
		client = cl
		mgr.clients[projectID] = cl
	}

	return client
}
