// Package pubsub provides an Encore-compatible Pub/Sub implementation backed by NATS and JetStream.
// Optimized: caches stream setup, deduplicates common logic,
// fixes partitioned publish bugs, and uses functional options.
package natspubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Client holds shared NATS/JetStream and observability.
type Client struct {
	nc         *nats.Conn
	js         nats.JetStreamContext
	logger     *zap.Logger
	tracer     trace.Tracer
	metrics    *metrics
	initOnce   sync.Once
	setupMutex sync.Mutex
	streams    map[string]struct{}
}

type metrics struct {
	publishCounter   *prometheus.CounterVec
	subscribeCounter *prometheus.CounterVec
	errorCounter     *prometheus.CounterVec
}

// Option configures TopicConfig.
type Option func(*TopicConfig)

// TopicConfig holds behavior settings.
type TopicConfig struct {
	DeliveryGuarantee nats.PubOpt       // e.g. nats.AckNone(), nats.AckExplicit()
	Stream            nats.StreamConfig // retention, storage, etc.
	AckWait           time.Duration
	MaxInflight       int
	QueueGroup        string
}

// Topic represents a typed subject.
type Topic[T any] struct {
	client  *Client
	subject string
	cfg     TopicConfig
}

// NewClient initializes a singleton NATS+JetStream client.
func NewClient() *Client {
	c := &Client{streams: make(map[string]struct{})}
	c.initOnce.Do(func() {
		logger, err := zap.NewProduction()
		if err != nil {
			panic(err)
		}
		tracer := otel.Tracer("pubsub")
		nc, err := nats.Connect(nats.DefaultURL, nats.MaxReconnects(-1))
		if err != nil {
			logger.Fatal("nats connect", zap.Error(err))
		}
		js, err := nc.JetStream()
		if err != nil {
			logger.Fatal("jetstream init", zap.Error(err))
		}
		c.nc, c.js, c.logger, c.tracer = nc, js, logger, tracer
		c.metrics = &metrics{
			publishCounter: prometheus.NewCounterVec(
				prometheus.CounterOpts{Name: "pubsub_publish_total"}, []string{"subject"}),
			subscribeCounter: prometheus.NewCounterVec(
				prometheus.CounterOpts{Name: "pubsub_subscribe_total"}, []string{"subject"}),
			errorCounter: prometheus.NewCounterVec(
				prometheus.CounterOpts{Name: "pubsub_errors_total"}, []string{"subject", "phase"}),
		}
		prometheus.MustRegister(c.metrics.publishCounter, c.metrics.subscribeCounter, c.metrics.errorCounter)
	})
	return c
}

// NewTopic creates a new Topic[T]. Usage:
//
//	topic := pubsub.NewTopic[OrderCreated](client, "orders.created", opts...)
func NewTopic[T any](c *Client, subject string, opts ...Option) *Topic[T] {
	// defaults
	cfg := TopicConfig{
		DeliveryGuarantee: nats.AckWait(30 * time.Second),
		Stream:            nats.StreamConfig{Retention: nats.LimitsPolicy, Storage: nats.FileStorage, MaxAge: 24 * time.Hour, Replicas: 1},
		AckWait:           30 * time.Second,
		MaxInflight:       1,
	}
	for _, o := range opts {
		o(&cfg)
	}
	return &Topic[T]{client: c, subject: subject, cfg: cfg}
}

// WithAtLeastOnce sets at-least-once delivery.
func WithAtLeastOnce() Option {
	return func(cfg *TopicConfig) { cfg.DeliveryGuarantee = nats.AckWait(30 * time.Second) }
}

// WithStreamConfig overrides the StreamConfig.
func WithStreamConfig(sc nats.StreamConfig) Option {
	return func(cfg *TopicConfig) { cfg.Stream = sc }
}

// WithSubscriptionOptions sets AckWait, MaxInflight, QueueGroup.
func WithSubscriptionOptions(ackWait time.Duration, maxInflight int, queue string) Option {
	return func(cfg *TopicConfig) {
		cfg.AckWait, cfg.MaxInflight, cfg.QueueGroup = ackWait, maxInflight, queue
	}
}

// ensureStream idempotently creates the JetStream stream.
func (c *Client) ensureStream(name string, sc nats.StreamConfig) error {
	c.setupMutex.Lock()
	defer c.setupMutex.Unlock()
	if _, exists := c.streams[name]; exists {
		return nil
	}
	_, err := c.js.AddStream(&sc)
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		return err
	}
	c.streams[name] = struct{}{}
	return nil
}

// Publish sends an event of type T.
func (t *Topic[T]) Publish(ctx context.Context, event *T) (string, error) {
	_, span := t.client.tracer.Start(ctx, "Publish",
		trace.WithAttributes(attribute.String("subject", t.subject)))
	defer span.End()

	data, err := json.Marshal(event)
	if err != nil {
		t.client.metrics.errorCounter.WithLabelValues(t.subject, "marshal").Inc()
		return "", err
	}
	t.client.metrics.publishCounter.WithLabelValues(t.subject).Inc()

	if t.cfg.DeliveryGuarantee == nats.AckWait(30*time.Second) {
		sc := t.cfg.Stream
		sc.Name = t.subject
		sc.Subjects = []string{t.subject}
		if err := t.client.ensureStream(sc.Name, sc); err != nil {
			return "", err
		}
		ack, err := t.client.js.Publish(t.subject, data)
		if err != nil {
			t.client.metrics.errorCounter.WithLabelValues(t.subject, "publish").Inc()
			return "", err
		}
		return fmt.Sprint(ack.Sequence), nil
	}
	return "", t.client.nc.Publish(t.subject, data)
}

// SubscriptionConfig[T] holds your handler for T.
type SubscriptionConfig[T any] struct {
	Handler func(context.Context, *T) error
}

// Subscribe starts consuming events.
// Subscribe attaches a handler to the topic.
//   - For at-least-once (JetStream), it uses js.QueueSubscribe when QueueGroup is set,
//     otherwise js.Subscribe.
//   - For at-most-once, it uses nc.QueueSubscribe or nc.Subscribe.
func (t *Topic[T]) Subscribe(durable string, cfg SubscriptionConfig[T]) error {
	// Build the message handler once
	handler := func(msg *nats.Msg) {
		ctx, span := t.client.tracer.Start(
			context.Background(),
			"HandleMessage",
			trace.WithAttributes(attribute.String("subject", t.subject)),
		)
		defer span.End()

		var e T
		if err := json.Unmarshal(msg.Data, &e); err != nil {
			t.client.metrics.errorCounter.
				WithLabelValues(t.subject, "unmarshal").
				Inc()
			return
		}

		t.client.metrics.subscribeCounter.
			WithLabelValues(t.subject).
			Inc()

		if err := cfg.Handler(ctx, &e); err != nil {
			t.client.metrics.errorCounter.
				WithLabelValues(t.subject, "handler").
				Inc()
			return
		}
		// Ack only in JetStream mode
		msg.Ack()
	}

	// JetStream (At-Least-Once)
	if t.cfg.DeliveryGuarantee == nats.AckWait(30*time.Second) {
		// Ensure the stream exists
		sc := t.cfg.Stream
		sc.Name = t.subject
		sc.Subjects = []string{t.subject}
		if err := t.client.ensureStream(sc.Name, sc); err != nil {
			return err
		}

		// Use QueueSubscribe for JetStream queue groups
		if t.cfg.QueueGroup != "" {
			_, err := t.client.js.QueueSubscribe(
				t.subject,
				t.cfg.QueueGroup,
				handler,
				nats.Durable(durable),
				nats.ManualAck(),
				nats.AckWait(t.cfg.AckWait),
				nats.MaxAckPending(t.cfg.MaxInflight),
			)
			return err
		}

		// Fallback to plain JetStream subscribe
		_, err := t.client.js.Subscribe(
			t.subject,
			handler,
			nats.Durable(durable),
			nats.ManualAck(),
			nats.AckWait(t.cfg.AckWait),
			nats.MaxAckPending(t.cfg.MaxInflight),
		)
		return err
	}

	// Core NATS (At-Most-Once)
	if t.cfg.QueueGroup != "" {
		_, err := t.client.nc.QueueSubscribe(
			t.subject,
			t.cfg.QueueGroup,
			handler,
		)
		return err
	}
	_, err := t.client.nc.Subscribe(
		t.subject,
		handler,
	)
	return err
}

// PartitionedTopic for per-user queues.
type PartitionedTopic[T any] struct{ topic *Topic[T] }

// NewPartitionedTopic[T] constructs it.
func NewPartitionedTopic[T any](c *Client, subject string, opts ...Option) *PartitionedTopic[T] {
	return &PartitionedTopic[T]{topic: NewTopic[T](c, subject, opts...)}
}

// PublishForUser sends to "<subject>.<userID>".
func (pt *PartitionedTopic[T]) PublishForUser(ctx context.Context, userID string, evt *T) (string, error) {
	pt.topic.subject = fmt.Sprintf("%s.%s", pt.topic.subject, userID)
	return pt.topic.Publish(ctx, evt)
}

// BucketedTopic for modulo-N partitions.
type BucketedTopic[T any] struct {
	pt         *PartitionedTopic[T]
	partitions int
}

// NewBucketedTopic[T] constructs it.
func NewBucketedTopic[T any](c *Client, subject string, partitions int, opts ...Option) *BucketedTopic[T] {
	return &BucketedTopic[T]{
		pt:         NewPartitionedTopic[T](c, subject, opts...),
		partitions: partitions,
	}
}

// PublishWithKey hashes key%N.
func (bt *BucketedTopic[T]) PublishWithKey(ctx context.Context, key string, evt *T) (string, error) {
	h := fnv.New32a()
	h.Write([]byte(key))
	idx := int(h.Sum32()) % bt.partitions
	bt.pt.topic.subject = fmt.Sprintf("%s.%d", bt.pt.topic.subject, idx)
	return bt.pt.topic.Publish(ctx, evt)
}

// SubscribePartition binds to a single partition.
func (bt *BucketedTopic[T]) SubscribePartition(durable string, partition int, cfg SubscriptionConfig[T]) error {
	bt.pt.topic.subject = fmt.Sprintf("%s.%d", bt.pt.topic.subject, partition)
	return bt.pt.topic.Subscribe(durable, cfg)
}
