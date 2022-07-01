package gcp

import (
	"fmt"
	"sync"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/iterator"

	"encore.dev/internal/ctx"
	"encore.dev/runtime/config"
)

var (
	clients     = make(map[string]*pubsub.Client)
	clientMutex sync.Mutex
)

// getClient returns a singleton pubsub client for the given project or panics if it cannot be created.
func getClient(project *config.GCPPubSubServer) *pubsub.Client {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	// Check if we have it already
	if client, found := clients[project.ID]; found {
		return client
	}

	// Create a new client
	client, err := pubsub.NewClient(ctx.App, project.ID)
	if err != nil {
		panic(fmt.Sprintf("failed to create pubsub client to GCP project %s: %s", project.ID, err))
	}
	clients[project.ID] = client

	// Force a simple API call through to test that our credentials work for this project
	// as the call to NewClient above doesn't actually try and talk to any GCP API
	_, err = client.Topics(ctx.App).Next()
	if err != iterator.Done {
		panic(fmt.Sprintf("pubsub client status call failed %s: %s", project.ID, err))
	}

	return client
}
