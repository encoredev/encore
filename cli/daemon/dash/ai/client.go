package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/hasura/go-graphql-client"
	"github.com/hasura/go-graphql-client/pkg/jsonutil"
	"github.com/rs/zerolog/log"

	"encr.dev/internal/conf"
)

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

func getClient(errHandler func(err error)) *graphql.SubscriptionClient {
	client := graphql.NewSubscriptionClient(conf.WSBaseURL + "/graphql").
		WithRetryTimeout(5 * time.Second).
		WithRetryDelay(2 * time.Second).
		WithRetryStatusCodes("500-599").
		WithWebSocketOptions(
			graphql.WebsocketOptions{
				HTTPClient: conf.AuthClient,
			}).WithSyncMode(true)
	go func() {
		log.Info().Msg("starting ai client")
		err := client.Run()
		log.Info().Msg("closed ai client")
		if err != nil {
			errHandler(err)
		}
	}()
	return client
}

type AITask struct {
	SubscriptionID string
	client         *graphql.SubscriptionClient
}

func (t *AITask) Stop() error {
	return t.client.Unsubscribe(t.SubscriptionID)
}

// startAITask is a helper function to intitiate an AI query to the encore platform. The query
// should be assembled to stream a 'result' graphql field that is a AIStreamMessage.
func startAITask[Query any](ctx context.Context, params map[string]interface{}, notifier AINotifier) (*AITask, error) {
	var subId string
	var errStrReply = func(error string, code any) error {
		log.Error().Msgf("ai error: %s (%v)", error, code)
		_ = notifier(ctx, &AINotification{
			SubscriptionID: subId,
			Error:          &AIError{Message: error, Code: fmt.Sprintf("%v", code)},
			Finished:       true,
		})
		return graphql.ErrSubscriptionStopped
	}
	var errReply = func(err error) error {
		var graphqlErr graphql.Errors
		if errors.As(err, &graphqlErr) {
			for _, e := range graphqlErr {
				_ = errStrReply(e.Message, e.Extensions["code"])
			}
			return graphql.ErrSubscriptionStopped
		}
		return errStrReply(err.Error(), "")
	}
	var query Query
	client := getClient(func(err error) { _ = errReply(err) })
	subId, err := client.Subscribe(&query, params, func(message []byte, err error) error {
		if err != nil {
			return errReply(err)
		}
		var result aiTask
		err = jsonutil.UnmarshalGraphQL(message, &result)
		if err != nil {
			return errReply(err)
		}
		if result.Message.Error != "" {
			return errStrReply(result.Message.Error, "")
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
	return &AITask{SubscriptionID: subId, client: client}, err
}

// AINotification is a wrapper around messages and errors from the encore platform ai service
type AINotification struct {
	SubscriptionID string   `json:"subscriptionId,omitempty"`
	Value          any      `json:"value,omitempty"`
	Error          *AIError `json:"error,omitempty"`
	Finished       bool     `json:"finished,omitempty"`
}

type AIError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type AINotifier func(context.Context, *AINotification) error
