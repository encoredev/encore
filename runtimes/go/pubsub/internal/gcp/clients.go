package gcp

import (
	"fmt"
	"runtime"

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

// numGoroutines computes the number of goroutines to use for the subscription,
// by adaptively taking into account gRPC stream limits and the number of subscriptions.
func numGoroutines(numSubs int) int {
	if numSubs < 1 {
		numSubs = 1 // avoid division by zero
	}

	maxProcs := runtime.GOMAXPROCS(0)
	numConns := min(4, maxProcs)
	maxStreams := numConns * 90

	// Clamp to [1, 10].
	return max(min(maxStreams/numSubs, 10), 1)
}
