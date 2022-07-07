package gcp

import (
	"fmt"
	"sync"

	"cloud.google.com/go/pubsub"

	"encore.dev/internal/ctx"
)

var (
	client      *pubsub.Client
	clientMutex sync.Mutex
)

// getClient returns a singleton pubsub client for the given project or panics if it cannot be created.
func getClient() *pubsub.Client {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	// Check if we have it already
	if client != nil {
		return client
	}

	// Create a new client
	client, err := pubsub.NewClient(ctx.App, "")
	if err != nil {
		panic(fmt.Sprintf("failed to create pubsub client: %s", err))
	}

	return client
}
