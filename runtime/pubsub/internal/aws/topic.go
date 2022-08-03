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
	"github.com/rs/zerolog"

	"encore.dev/appruntime/config"
	"encore.dev/pubsub/internal/types"
)

type topic struct {
	ctx       context.Context
	snsClient *sns.Client
	sqsClient *sqs.Client
	cfg       *config.PubsubTopic
}

var _ types.TopicImplementation = (*topic)(nil)

func (t *topic) PublishMessage(ctx context.Context, attrs map[string]string, data []byte) (id string, err error) {
	attributes := make(map[string]snsTypes.MessageAttributeValue)
	for key, value := range attrs {
		attributes[key] = snsTypes.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(value),
		}
	}

	result, err := t.snsClient.Publish(ctx, &sns.PublishInput{
		Message:           aws.String(string(data)),
		MessageAttributes: attributes,
		TopicArn:          aws.String(t.cfg.ProviderName),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(result.MessageId), nil
}

func (t *topic) Subscribe(logger *zerolog.Logger, ackDeadline time.Duration, _ *types.RetryPolicy, implCfg *config.PubsubSubscription, f types.RawSubscriptionCallback) {
	// Check we have permissions to interact with the given queue
	// otherwise the first time we will find out is when we try and publish to it
	_, err := t.sqsClient.GetQueueAttributes(t.ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(implCfg.ProviderName),
	})
	if err != nil {
		panic(fmt.Sprintf("unable to verify SQS queue attributes (may be missing IAM role allowing access): %v", err))
	}

	go func() {
		for t.ctx.Err() == nil {
			resp, err := t.sqsClient.ReceiveMessage(t.ctx, &sqs.ReceiveMessageInput{
				QueueUrl:              aws.String(implCfg.ProviderName),
				AttributeNames:        []sqsTypes.QueueAttributeName{"ApproximateReceiveCount"},
				MaxNumberOfMessages:   1, // We only pull 1 message at a time, as the ackDeadline is per message
				MessageAttributeNames: []string{"All"},
				VisibilityTimeout:     int32(ackDeadline.Seconds()),
				WaitTimeSeconds:       20, // Maximum allowed time
			})

			if err != nil && t.ctx.Err() == nil {
				logger.Warn().Err(err).Msg("pubsub subscription failed, retrying in 5 seconds")
				time.Sleep(5 * time.Second)
				continue
			}

			for _, msg := range resp.Messages {
				// Parse the message body
				msgWrapper := &SNSMessageWrapper{}
				if err := json.Unmarshal([]byte(aws.ToString(msg.Body)), msgWrapper); err != nil {
					logger.Err(err).Str("sqs_msg_id", aws.ToString(msg.MessageId)).Msg("unable to parse message")
					continue
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
				err = f(t.ctx, msgWrapper.MessageId, msgWrapper.Timestamp, int(deliveryAttempt), attributes, []byte(msgWrapper.Message))
				if err == nil {
					_, err = t.sqsClient.DeleteMessage(t.ctx, &sqs.DeleteMessageInput{
						QueueUrl:      aws.String(implCfg.ProviderName),
						ReceiptHandle: msg.ReceiptHandle,
					})
					if err != nil {
						logger.Err(err).Str("msg_id", msgWrapper.MessageId).Msg("unable to delete message from SQS queue")
					}
				}
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
