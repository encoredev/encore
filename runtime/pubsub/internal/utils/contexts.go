package utils

import (
	"context"
)

// Contexts is a struct containing all the contexts used by the pubsub package
type Contexts struct {
	// Fetch is the context used for fetching messages from the pubsub provider
	//
	// It is cancelled when the manager is shutdown and is used to indicate that
	// no more messages should be fetched from the provider.
	//
	// The fetch context is always the first context to be cancelled.
	Fetch                 context.Context
	StopFetchingNewEvents context.CancelFunc

	// Handler is the context used for handling messages from the pubsub provider
	//
	// It is cancelled when the manager is told to stop any active handlers.
	//
	// If cancelled before the fetch context, it will also cancel the fetch context.
	Handler       context.Context
	CancelHandler context.CancelFunc

	// Connection is the context used for the connection to the pubsub provider
	//
	// If cancelled, it will cancel both the fetch and handler contexts.
	Connection       context.Context
	CloseConnections context.CancelFunc
}

// NewContexts creates a new set of contexts for the pubsub package
func NewContexts(base context.Context) *Contexts {
	connection, cancelConnection := context.WithCancel(base)
	handler, cancelHandler := context.WithCancel(connection)
	fetch, cancelFetch := context.WithCancel(handler)

	ctxs := &Contexts{
		Fetch:                 fetch,
		StopFetchingNewEvents: cancelFetch,
		Handler:               handler,
		CancelHandler:         cancelHandler,
		Connection:            connection,
		CloseConnections:      cancelConnection,
	}

	return ctxs
}
