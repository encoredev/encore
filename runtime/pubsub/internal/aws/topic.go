package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snsTypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqsTypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/rs/xid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encore.dev/appruntime/exported/config"
	"encore.dev/pubsub/internal/types"
	"encore.dev/pubsub/internal/utils"
)

type topic struct {
	ctx         context.Context
	publisherID xid.ID
	snsClient   *sns.Client
	sqsClient   *sqs.Client
	staticCfg   types.TopicConfig
	runtimeCfg  *config.PubsubTopic
}

var _ types.TopicImplementation = (*topic)(nil)

func (t *topic) PublishMessage(ctx context.Context, orderingKey string, attrs map[string]string, data []byte) (id string, err error) {
	attributes := make(map[string]snsTypes.MessageAttributeValue)
	for key, value := range attrs {
		attributes[key] = snsTypes.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(value),
		}
	}

	params := &sns.PublishInput{
		Message:           aws.String(string(data)),
		MessageAttributes: attributes,
		TopicArn:          aws.String(t.runtimeCfg.ProviderName),
	}

	// For exactly-once delivery on AWS we need to:
	//
	// 1. Set a message group ID (as this is a requirement for FIFO queues)
	// 2. Set a message deduplication ID as this is required to enable exactly-once delivery
	if t.staticCfg.DeliveryGuarantee == types.ExactlyOnce {
		params.MessageGroupId = aws.String(fmt.Sprintf("inst_%s", t.publisherID.String()))
		params.MessageDeduplicationId = aws.String(fmt.Sprintf("msg_%s", xid.New().String()))
	}

	// If we have an explicit ordering key, use that as the message group ID and mark the topic as FIFO
	if orderingKey != "" {
		params.MessageGroupId = aws.String(orderingKey)
		params.MessageDeduplicationId = aws.String(fmt.Sprintf("msg_%s", xid.New().String()))
	}

	result, err := t.snsClient.Publish(ctx, params)
	if err != nil {
		return "", err
	}
	return aws.ToString(result.MessageId), nil
}

func (t *topic) Subscribe(logger *zerolog.Logger, maxConcurrency int, ackDeadline time.Duration, retryPolicy *types.RetryPolicy, implCfg *config.PubsubSubscription, f types.RawSubscriptionCallback) {
	// Check we have permissions to interact with the given queue
	// otherwise the first time we will find out is when we try and publish to it
	_, err := t.sqsClient.GetQueueAttributes(t.ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(implCfg.ProviderName),
	})
	if err != nil {
		panic(fmt.Sprintf("unable to verify SQS queue attributes (may be missing IAM role allowing access): %v", err))
	}

	ackDeadline = utils.Clamp(ackDeadline, time.Second, 12*time.Hour)

	if maxConcurrency == 0 {
		maxConcurrency = 1 // FIXME(domblack): This retains the old behaviour, but allows user customisation - in a future release we should remove this
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error().Interface("panic", r).Msg("panic in subscriber, no longer processing messages")
			} else {
				logger.Info().Msg("subscriber stopped due to context cancellation")
			}
		}()

		for t.ctx.Err() == nil {
			err := utils.WorkConcurrently(
				t.ctx,
				maxConcurrency, 10,
				func(ctx context.Context, maxToFetch int) ([]sqsTypes.Message, error) {
					// We should only long poll for 20 seconds, so if this takes more than
					// 30 seconds we should cancel the context and try again
					//
					// We do this incase the ReceiveMessage call gets stuck on the server
					// and doesn't return
					ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()

					resp, err := t.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
						QueueUrl:              aws.String(implCfg.ProviderName),
						AttributeNames:        []sqsTypes.QueueAttributeName{"ApproximateReceiveCount"},
						MaxNumberOfMessages:   int32(maxToFetch),
						MessageAttributeNames: []string{"All"},
						VisibilityTimeout:     int32(ackDeadline.Seconds()),
						WaitTimeSeconds:       20, // Maximum allowed time
					})
					if err != nil {
						return nil, err
					}

					return resp.Messages, nil
				},
				func(ctx context.Context, msg sqsTypes.Message) error {
					// Parse the message body
					msgWrapper := &SNSMessageWrapper{}
					if err := json.Unmarshal([]byte(aws.ToString(msg.Body)), msgWrapper); err != nil {
						logger.Err(err).Str("sqs_msg_id", aws.ToString(msg.MessageId)).Msg("unable to parse message")
						return nil
					}

					// Get the delivery attempt number
					deliveryAttempt, err := parseInt(msg.Attributes, "ApproximateReceiveCount")
					if err != nil {
						logger.Warn().Err(err).Str("msg_id", msgWrapper.MessageId).Msg("unable to parse receive count")
					}

					// Extract the attributes
					attributes := make(map[string]string)
					for key, value := range msgWrapper.MessageAttributes {
						switch value.Type {
						case "String":
							attributes[key] = value.Value
						default:
							logger.Warn().Err(err).Str("msg_id", msgWrapper.MessageId).Str("attr_name", key).Str("attr_type", value.Type).Msg("unsupported attribute data type")
						}
					}

					// Call the callback, and if there was no error, then we can delete the message
					msgCtx, cancel := context.WithTimeout(ctx, ackDeadline)
					defer cancel()
					err = f(msgCtx, msgWrapper.MessageId, msgWrapper.Timestamp, int(deliveryAttempt), attributes, []byte(msgWrapper.Message))
					cancel()

					// Check if the context has been cancelled, and if so, return the error
					if ctx.Err() != nil {
						return ctx.Err()
					}

					// We want to wait a maximum of 30 seconds for the callback to complete
					// otherwise we assume it has failed and we should retry
					//
					// We do this incase the callback gets stuck and doesn't return
					ctx, responseCancel := context.WithTimeout(ctx, 30*time.Second)
					defer responseCancel()

					if err != nil {
						logger.Err(err).Str("msg_id", msgWrapper.MessageId).Msg("unable to process message")

						// If there was an error processing the message, apply the backoff policy
						_, delay := utils.GetDelay(retryPolicy.MaxRetries, retryPolicy.MinBackoff, retryPolicy.MaxBackoff, uint16(deliveryAttempt))
						_, visibilityChangeErr := t.sqsClient.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
							QueueUrl:          aws.String(implCfg.ProviderName),
							ReceiptHandle:     msg.ReceiptHandle,
							VisibilityTimeout: int32(utils.Clamp(delay, time.Second, 12*time.Hour).Seconds()),
						})
						if visibilityChangeErr != nil {
							log.Warn().Err(visibilityChangeErr).Str("msg_id", msgWrapper.MessageId).Msg("unable to change message visibility to apply backoff rules")
						}
					} else {
						// If the message was processed successfully, delete it from the queue
						_, err = t.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
							QueueUrl:      aws.String(implCfg.ProviderName),
							ReceiptHandle: msg.ReceiptHandle,
						})
						if err != nil {
							logger.Err(err).Str("msg_id", msgWrapper.MessageId).Msg("unable to delete message from SQS queue")
						}
					}

					return nil
				},
			)

			if err != nil && t.ctx.Err() == nil {
				logger.Warn().Err(err).Msg("pubsub subscription failed, retrying in 5 seconds")
				time.Sleep(5 * time.Second)
				continue
			}
		}
	}()
}

func parseInt(m map[string]string, key string) (int64, error) {
	value, ok := m[key]
	if !ok {
		return 0, errors.New("attribute was not received")
	}

	return strconv.ParseInt(value, 10, 64)
}

// SNSMessageWrapper matches the JSON that is sent to SQS from an SNS subscription
type SNSMessageWrapper struct {
	Type              string    `json:"Type"`
	MessageId         string    `json:"MessageId"`
	TopicArn          string    `json:"TopicArn"`
	Message           string    `json:"Message"`
	Timestamp         time.Time `json:"Timestamp"`
	SignatureVersion  string    `json:"SignatureVersion"`
	SigningCertURL    string    `json:"SigningCertURL"`
	UnsubscribeURL    string    `json:"UnsubscribeURL"`
	MessageAttributes map[string]struct {
		Type  string `json:"Type"`
		Value string `json:"Value"`
	} `json:"MessageAttributes"`
}
