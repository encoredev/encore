package aws

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/rs/zerolog/log"

	"encore.dev/appruntime/exported/config"
	"encore.dev/pubsub/internal/types"
	"encore.dev/pubsub/internal/utils"
)

const (
	testTopicARN = "arn:aws:sns:us-west-2:406859400861:test-app-1-test-env-1-doms-test-topic"
	testQueueURL = "https://sqs.us-west-2.amazonaws.com/406859400861/test-app-1-test-env-1-doms-test-topic_test-subscriber"
)

func Test_AWS_PubSub_E2E(t *testing.T) {
	// Skip this test if the access keys needed are not set
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("AWS_ACCESS_KEY_ID is not set")
	}
	if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		t.Skip("AWS_SECRET_ACCESS_KEY is not set")
	}

	runtime := &config.Runtime{
		PubsubProviders: []*config.PubsubProvider{
			{AWS: &config.AWSPubsubProvider{}},
		},
		PubsubTopics: map[string]*config.PubsubTopic{
			"test-topic": {
				EncoreName:   "test-topic",
				ProviderID:   0,
				ProviderName: testTopicARN,
				Subscriptions: map[string]*config.PubsubSubscription{
					"test-subscription": {
						EncoreName:   "test-subscription",
						ProviderName: testQueueURL,
					},
				},
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	ctxs := utils.NewContexts(ctx)
	defer cancel()
	mgr := NewManager(ctxs)

	topic := mgr.NewTopic(runtime.PubsubProviders[0], types.TopicConfig{DeliveryGuarantee: types.AtLeastOnce}, runtime.PubsubTopics["test-topic"])

	// Purge the queue of any messages from previous failed tests
	_, err := mgr.getSQSClient(ctx).PurgeQueue(ctx, &sqs.PurgeQueueInput{
		QueueUrl: aws.String(testQueueURL),
	})
	if err != nil {
		t.Fatalf("failed to purge queue: %s", err)
	}

	// Subscribe to the queue
	msgChan := make(chan string)
	var sentMessageID string
	topic.Subscribe(&log.Logger, 0, time.Second, nil, runtime.PubsubTopics["test-topic"].Subscriptions["test-subscription"], func(ctx context.Context, msgID string, publishTime time.Time, deliveryAttempt int, attrs map[string]string, data []byte) error {
		if attrs["attr-1"] != "foo" {
			t.Errorf("expected attr-1 to be foo, got %s", attrs["attr-1"])
		}
		if msgID != sentMessageID {
			t.Errorf("expected message ID to be %s, got %s", sentMessageID, msgID)
		}
		if deliveryAttempt != 1 {
			t.Errorf("expected delivery attempt to be 1, got %d", deliveryAttempt)
		}
		if publishTime.Before(time.Now().Add(-1 * time.Minute)) {
			t.Errorf("expected publish time to be within the last minute, got %s", publishTime)
		}
		msgChan <- string(data)
		return nil
	})

	// Publish a message on the queue
	sentMessageID, err = topic.PublishMessage(context.Background(), "", map[string]string{"attr-1": "foo"}, []byte("{\"hello\":\"world\"}"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify receipt of the message
	select {
	case msg := <-msgChan:
		if msg != "{\"hello\":\"world\"}" {
			t.Errorf("expected message to be {\"hello\":\"world\"}, got %s", msg)
		}

		// Sleep to allow time for the subscription go routine to delete the message
		time.Sleep(1 * time.Second)
	case <-ctx.Done():
		t.Errorf("timed out waiting for message")
	}
}
