package ai

import (
	"context"
	"sync"

	"github.com/hasura/go-graphql-client"
	"github.com/hasura/go-graphql-client/pkg/jsonutil"
)

func newLazyClient(client *graphql.SubscriptionClient) *LazySubClient {
	lazy := &LazySubClient{
		SubscriptionClient: client,
		notifiers:          make(map[string]func([]byte, error) error),
	}
	client.OnDisconnected(func() {
		lazy.mu.Lock()
		defer lazy.mu.Unlock()
		lazy.running = false
	})
	client.OnConnected(func() {
		lazy.mu.Lock()
		defer lazy.mu.Unlock()
		lazy.running = true
	})
	client.OnSubscriptionComplete(func(sub graphql.Subscription) {
		lazy.mu.Lock()
		defer lazy.mu.Unlock()
		delete(lazy.notifiers, sub.GetKey())
	})
	return lazy
}

// LazySubClient is a wrapper around graphql.SubscriptionClient that starts the client when the first subscription is made.
// It also stops the client when the last subscription is removed and reconnects when a subscription is added.
type LazySubClient struct {
	*graphql.SubscriptionClient

	mu        sync.Mutex
	running   bool
	notifiers map[string]func([]byte, error) error
}

func (l *LazySubClient) Subscribe(query interface{}, variables map[string]interface{}, notify func([]byte, error) error) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running {
		go func() {
			defer l.Close()
			err := l.Run()
			l.mu.Lock()
			defer l.mu.Unlock()
			if err != nil {
				for _, n := range l.notifiers {
					_ = n(nil, err)
				}
			}
			l.notifiers = make(map[string]func([]byte, error) error)
		}()
	}
	subID, err := l.SubscriptionClient.Subscribe(query, variables, notify)
	if err != nil {
		return "", err
	}
	key := l.GetSubscription(subID).GetKey()
	l.notifiers[key] = notify
	return subID, nil
}

func query[T any](ctx context.Context, c *LazySubClient, params map[string]interface{}, notifier AINotifier) (string, error) {
	var subId string
	var errStrReply = func(error string) error {
		_ = notifier(ctx, &WSNotification{
			SubscriptionID: subId,
			Error:          error,
			Finished:       true,
		})
		return graphql.ErrSubscriptionStopped
	}
	var errReply = func(err error) error {
		return errStrReply(err.Error())
	}
	var query T
	subId, err := c.Subscribe(&query, params, func(message []byte, err error) error {
		if err != nil {
			return errReply(err)
		}
		var result genericQuery
		err = jsonutil.UnmarshalGraphQL(message, &result)
		if err != nil {
			return errReply(err)
		}
		if result.StreamUpdate.Error != "" {
			return errStrReply(result.StreamUpdate.Error)
		}
		err = notifier(ctx, &WSNotification{
			SubscriptionID: subId,
			Value:          result.StreamUpdate.Value.GetValue(),
			Finished:       result.StreamUpdate.Finished,
		})
		if err != nil {
			return errReply(err)
		}

		return nil
	})
	c.GetSubscription(subId).GetKey()
	return subId, err
}

type UpdateQuery struct {
	Type string `graphql:"__typename"`

	ServiceUpdate   `graphql:"... on ServiceUpdate"`
	TypeUpdate      `graphql:"... on TypeUpdate"`
	TypeFieldUpdate `graphql:"... on TypeFieldUpdate"`
	ErrorUpdate     `graphql:"... on ErrorUpdate"`
	EndpointUpdate  `graphql:"... on EndpointUpdate"`
	SessionUpdate   `graphql:"... on SessionUpdate"`
	TitleUpdate     `graphql:"... on TitleUpdate"`
	PathParamUpdate `graphql:"... on PathParamUpdate"`
}

func (u *UpdateQuery) GetValue() AIUpdateType {
	switch u.Type {
	case "ServiceUpdate":
		return u.ServiceUpdate
	case "TypeUpdate":
		return u.TypeUpdate
	case "TypeFieldUpdate":
		return u.TypeFieldUpdate
	case "ErrorUpdate":
		return u.ErrorUpdate
	case "EndpointUpdate":
		return u.EndpointUpdate
	case "SessionUpdate":
		return u.SessionUpdate
	case "TitleUpdate":
		return u.TitleUpdate
	case "PathParamUpdate":
		return u.PathParamUpdate
	}
	return nil
}

type StreamUpdate struct {
	Value    UpdateQuery
	Error    string
	Finished bool
}

type genericQuery struct {
	StreamUpdate *StreamUpdate `graphql:"result"`
}

type WSNotification struct {
	SubscriptionID string `json:"subscriptionId,omitempty"`
	Value          any    `json:"value,omitempty"`
	Error          string `json:"error,omitempty"`
	Finished       bool   `json:"finished,omitempty"`
}

type AINotifier func(context.Context, *WSNotification) error
