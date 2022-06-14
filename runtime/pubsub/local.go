//go:build encore_local

package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/nsqio/go-nsq"

	"encore.dev/pubsub/utils"
	"encore.dev/rlog"
	"encore.dev/runtime/config"
)

// localTopic is the nsq implementation of pubsub.Topic. It exposes methods to publish
// and subscribe to messages of a topic
type localTopic[T any] struct {
	name      string
	cfg       *TopicConfig
	addr      string
	producer  *nsq.Producer
	consumers map[string]*nsq.Consumer
	idSeq     uint32
}

// message is a local representation of a topic published to NSQ.
// it wraps the raw data with an ID and an Attribute map
type message[T any] struct {
	ID         string
	Attributes map[string]string
	Data       T
}

// handler implements the interface of an nsq.Hander. It is used to Unmarshal a message
// and forward it to a subscriber.
type handler[T any] struct {
	sub Subscriber[T]
	cfg *SubscriptionConfig
}

// HandleMessage unmarshals a message into the message type and forwards it to the subscriber
func (h *handler[T]) HandleMessage(m *nsq.Message) (err error) {
	defer func() {
		if err != nil {
			rlog.Error("failed to handle message", "messageId", m.ID, "err", err)
		}
		if !m.HasResponded() {
			policy := h.cfg.RetryPolicy
			retry, delay := utils.GetDelay(policy.MaxRetries, policy.MinRetryDelay, policy.MaxRetryDelay, m.Attempts)
			if !retry {
				rlog.Info("depleted message retries. Dropping message", "messageId", m.ID)
				m.Finish()
				return
			}
			m.RequeueWithoutBackoff(delay)
		}
	}()
	// create a message to unmarshal the raw nsq body into
	msg := &message[T]{}
	err = json.Unmarshal(m.Body, msg)
	if err != nil {
		return err
	}
	// unmarshal the attributes and write them to the message type
	err = utils.UnmarshalFields(msg.Attributes, &msg.Data, "pubsub-attr")
	if err != nil {
		return err
	}
	// forward the message to the subscriber
	err = h.sub(context.Background(), msg.Data)
	if err != nil {
		return err
	}
	m.Finish()
	return nil
}

var _ nsq.Handler = &handler[any]{}

// NewSubscription creates a subscription for a nsq topic
func (l *localTopic[T]) NewSubscription(name string, sub Subscriber[T], cfg *SubscriptionConfig) Subscription[T] {
	if _, ok := l.consumers[name]; ok {
		panic("NewSubscription must use a unique subscription name")
	}
	conCfg := nsq.NewConfig()
	consumer, err := nsq.NewConsumer(l.name, name, conCfg)
	if err != nil {
		panic(fmt.Sprintf("Error: %v", err))
	}
	// drop log messages for now since we cannot format them in the common daemon format
	consumer.SetLoggerLevel(nsq.LogLevelMax)
	// create a dedicated handler which forwards messages to the encore subscription
	consumer.AddHandler(&handler[T]{sub: sub, cfg: cfg})
	// connect the consumer to the NSQD
	err = consumer.ConnectToNSQD(l.addr)
	if err != nil {
		panic(fmt.Sprintf("Error: %v", err))
	}
	// add the consumer to the known consumers
	l.consumers[name] = consumer
	return consumer
}

// Publish publishes a message to an nsq Topic
func (l *localTopic[T]) Publish(ctx context.Context, msg T) (id string, err error) {
	// instantiate a Producer if there isn;t one already
	if l.producer == nil {
		cfg := nsq.NewConfig()
		producer, err := nsq.NewProducer(l.addr, cfg)
		if err != nil {
			return "", err
		}
		l.producer = producer
	}
	// generate a new message ID
	idx := fmt.Sprint(atomic.AddUint32(&l.idSeq, 1))

	// turn the attributes into an attribute map
	attrs, err := utils.MarshalFields(msg, "pubsub-attr")
	if err != nil {
		return "", err
	}
	// create and publish the message wrapper
	data, err := json.Marshal(&message[T]{ID: idx, Data: msg, Attributes: attrs})
	if err != nil {
		return "", err
	}
	err = l.producer.Publish(l.name, data)
	if err != nil {
		return "", err
	}
	// return the message id!
	return idx, nil
}

var _ Topic[any] = &localTopic[any]{}

// NewTopic is used to declare a Topic. Encore will use static
// analysis to identify Topics and automatically provision them
// for you.
func NewTopic[T any](name string, cfg *TopicConfig) Topic[T] {
	// fetch the topic configuration
	topic, ok := config.Cfg.Runtime.PubsubTopics[name]
	if !ok {
		panic("unregistered/unknown topic: " + name)
	}
	id := topic.ServerID
	if id >= len(config.Cfg.Runtime.PubsubServers) {
		panic(fmt.Sprintf("invalid PubsubServer id: %v", id))
	}
	// fetch the nsq server address
	address := config.Cfg.Runtime.PubsubServers[id].NSQServer.Address
	// return a topic instance!
	return &localTopic[T]{
		name:      name,
		cfg:       cfg,
		consumers: make(map[string]*nsq.Consumer),
		addr:      address,
	}
}
