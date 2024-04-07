package ai

import (
	"context"
	"sync"

	"github.com/hasura/go-graphql-client"
	"github.com/hasura/go-graphql-client/pkg/jsonutil"

	"encr.dev/pkg/fns"
)

// newLazySubClient wraps a graphql.SubscriptionClient and starts it when the first subscription is made.
func newLazySubClient(client *graphql.SubscriptionClient) *LazySubClient {
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

// Subscribe subscribes to a query and calls notify with the result.
func (l *LazySubClient) Subscribe(query interface{}, variables map[string]interface{}, notify func([]byte, error) error) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running {
		go func() {
			defer fns.CloseIgnore(l)
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

type TaskMessage struct {
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

func (u *TaskMessage) GetValue() AIUpdateType {
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

type AIStreamMessage struct {
	Value    TaskMessage
	Error    string
	Finished bool
}

type aiTask struct {
	Message *AIStreamMessage `graphql:"result"`
}

// startAITask is a helper function to intitiate an AI query to the encore platform. The query
// should be assembled to stream a 'result' graphql field that is a AIStreamMessage.
func startAITask[Query any](ctx context.Context, c *LazySubClient, params map[string]interface{}, notifier AINotifier) (string, error) {
	var subId string
	var errStrReply = func(error string) error {
		_ = notifier(ctx, &AINotification{
			SubscriptionID: subId,
			Error:          error,
			Finished:       true,
		})
		return graphql.ErrSubscriptionStopped
	}
	var errReply = func(err error) error {
		return errStrReply(err.Error())
	}
	var query Query
	subId, err := c.Subscribe(&query, params, func(message []byte, err error) error {
		if err != nil {
			return errReply(err)
		}
		var result aiTask
		err = jsonutil.UnmarshalGraphQL(message, &result)
		if err != nil {
			return errReply(err)
		}
		if result.Message.Error != "" {
			return errStrReply(result.Message.Error)
		}
		err = notifier(ctx, &AINotification{
			SubscriptionID: subId,
			Value:          result.Message.Value.GetValue(),
			Finished:       result.Message.Finished,
		})
		if err != nil {
			return errReply(err)
		}
		return nil
	})
	return subId, err
}

// AINotification is a wrapper around messages and errors from the encore platform ai service
type AINotification struct {
	SubscriptionID string `json:"subscriptionId,omitempty"`
	Value          any    `json:"value,omitempty"`
	Error          string `json:"error,omitempty"`
	Finished       bool   `json:"finished,omitempty"`
}

type AINotifier func(context.Context, *AINotification) error
