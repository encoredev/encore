package test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/testsupport"
	"encore.dev/pubsub/internal/types"
	"encore.dev/pubsub/internal/utils"
)

// TestTopic is used during a "encore test" call.
//
// It records all published messages on a per-test basis, allowing a unit test
// to assert that the correct messages were published.
//
// Any messages published to this type of topic _will not_ be passed to subscribers.
type TestTopic[T any] struct {
	ts          *testsupport.Manager
	name        string
	m           sync.RWMutex
	instances   map[*testing.T]*testInstance[T]
	subscribers map[string]types.RawSubscriptionCallback
}

func NewTopic[T any](ts *testsupport.Manager, name string) types.TopicImplementation {
	return &TestTopic[T]{
		ts:          ts,
		name:        name,
		instances:   make(map[*testing.T]*testInstance[T]),
		subscribers: make(map[string]types.RawSubscriptionCallback),
	}
}

// PublishMessage will record the message against the test instance
// and if subscribers are enabled for the test instance, it will also trigger
// those subscribers. (The default behaviour is subscribers are disabled in tests)
func (t *TestTopic[T]) PublishMessage(ctx context.Context, attrs map[string]string, data []byte) (id string, err error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	test := t.ts.CurrentTest()
	unmarshalled, err := utils.UnmarshalMessage[T](attrs, data)
	if err != nil {
		test.Fatalf("failed to unmarshal published message: %s", err)
	}

	instance := t.TestInstance(test)

	msgID, err := instance.publishMessage(unmarshalled)
	if err != nil {
		return "", err
	}

	// If subscriptions are enabled for this test, then trigger those subscribers asynchronously
	// allowing the publishing code to continue as it would in a real system
	if instance.subscriptionsEnabled {
		published := time.Now()

		for name, sub := range t.subscribers {
			name := name
			sub := sub
			t.ts.RunAsyncCodeInTest(test, func(ctx context.Context) {
				if err := sub(ctx, msgID, published, 1, attrs, data); err != nil {
					test.Errorf("an error was returned while processing subscription %s for message %s: %s", name, msgID, err)
					test.Fail()
				}
			})
		}
	}

	return msgID, nil
}

// Subscribe will register a new subscriber for the pub sub topic. By default these will not be called during tests
func (t *TestTopic[T]) Subscribe(logger *zerolog.Logger, ackDeadline time.Duration, retryPolicy *types.RetryPolicy, implCfg *config.PubsubSubscription, f types.RawSubscriptionCallback) {
	t.m.Lock()
	defer t.m.Unlock()
	t.subscribers[implCfg.EncoreName] = f
}

// TestInstance returns this tests specific instance of the topic and creates it if it does not exist
func (t *TestTopic[T]) TestInstance(test *testing.T) *testInstance[T] {
	t.m.RLock()
	instance, found := t.instances[test]
	t.m.RUnlock()
	if found {
		return instance
	}

	t.m.Lock()
	defer t.m.Unlock()
	if _, found := t.instances[test]; !found {
		t.instances[test] = &testInstance[T]{
			topicName: t.name,
			t:         test,
		}
	}

	return t.instances[test]
}

// testInstance represents a topic, as it is seen from a test
// This struct implements test.TestTopic[T] to allow the testing package to interface with it
type testInstance[T any] struct {
	topicName            string     // The topic name
	t                    *testing.T // The test we're running against
	msgID                int32      // The last message ID we sent (updated atomically)
	m                    sync.Mutex // Mutex for the published messages
	messages             []T        // What messages have been published
	subscriptionsEnabled bool       // If subscriptions are enabled for this test
}

// publishMessage records the message which was sent, and generates a deterministic message ID
// which is guaranteed to be unique across all tests
func (t *testInstance[T]) publishMessage(unmarshalled T) (id string, err error) {
	msgID := atomic.AddInt32(&t.msgID, 1)

	t.m.Lock()
	defer t.m.Unlock()
	t.messages = append(t.messages, unmarshalled)

	// we use "/" as the separator to mirror the behaviour of tests and sub tests
	return fmt.Sprintf("%s/%s/%d", t.t.Name(), t.topicName, msgID), nil
}

func (t *testInstance[T]) PublishedMessages() []T {
	t.m.Lock()
	defer t.m.Unlock()
	return t.messages
}
