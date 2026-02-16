// Package natspubsub provides an Encore-friendly Pub/Sub scaffolding backed by NATS and JetStream.
//
// The package intentionally stays lightweight: it focuses on typed publish/subscribe helpers,
// safe stream provisioning defaults, and operational guardrails around metrics/acking behavior.
package natspubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	defaultTracerName = "pubsub"
	defaultStreamTTL  = 24 * time.Hour
)

var streamNameSanitizer = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// DeliveryGuarantee controls delivery semantics.
type DeliveryGuarantee int

const (
	// AtMostOnce uses core NATS pub/sub semantics.
	AtMostOnce DeliveryGuarantee = iota
	// AtLeastOnce uses JetStream (durable persistence + manual acking).
	AtLeastOnce
)

// Client holds shared NATS/JetStream and observability handles.
type Client struct {
	nc      *nats.Conn
	js      nats.JetStreamContext
	logger  *zap.Logger
	tracer  trace.Tracer
	metrics *metrics

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
	DeliveryGuarantee DeliveryGuarantee
	Stream            nats.StreamConfig

	// Optional explicit stream identity. If left empty, defaults are derived from subject.
	StreamName     string
	StreamSubjects []string

	AckWait     time.Duration
	MaxInflight int
	QueueGroup  string
}

// Topic represents a typed subject.
type Topic[T any] struct {
	client *Client

	subject string

	streamName     string
	streamSubjects []string

	cfg TopicConfig
}

// NewClient initializes a NATS + JetStream client using sensible runtime defaults.
func NewClient() *Client {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	nc, err := nats.Connect(nats.DefaultURL, nats.MaxReconnects(-1))
	if err != nil {
		logger.Fatal("nats connect", zap.Error(err))
	}

	js, err := nc.JetStream()
	if err != nil {
		logger.Fatal("jetstream init", zap.Error(err))
	}

	return &Client{
		nc:      nc,
		js:      js,
		logger:  logger,
		tracer:  otel.Tracer(defaultTracerName),
		metrics: newMetrics(),
		streams: make(map[string]struct{}),
	}
}

// Close drains and closes the underlying NATS connection.
func (c *Client) Close() error {
	if c == nil || c.nc == nil {
		return nil
	}
	if err := c.nc.Drain(); err != nil {
		c.nc.Close()
		return err
	}
	c.nc.Close()
	return nil
}

// NewTopic creates a new Topic[T]. Usage:
//
//	topic := natspubsub.NewTopic[OrderCreated](client, "orders.created", opts...)
func NewTopic[T any](c *Client, subject string, opts ...Option) *Topic[T] {
	cfg := TopicConfig{
		DeliveryGuarantee: AtLeastOnce,
		Stream: nats.StreamConfig{
			Retention: nats.LimitsPolicy,
			Storage:   nats.FileStorage,
			MaxAge:    defaultStreamTTL,
			Replicas:  1,
		},
		AckWait:     30 * time.Second,
		MaxInflight: 1,
	}
	for _, o := range opts {
		o(&cfg)
	}

	streamName := cfg.StreamName
	if streamName == "" {
		streamName = cfg.Stream.Name
	}
	if streamName == "" {
		streamName = defaultStreamName(subject)
	}

	streamSubjects := append([]string(nil), cfg.StreamSubjects...)
	if len(streamSubjects) == 0 && len(cfg.Stream.Subjects) > 0 {
		streamSubjects = append([]string(nil), cfg.Stream.Subjects...)
	}
	if len(streamSubjects) == 0 {
		streamSubjects = []string{subject}
	}

	return &Topic[T]{
		client:         c,
		subject:        subject,
		streamName:     streamName,
		streamSubjects: streamSubjects,
		cfg:            cfg,
	}
}

// WithAtLeastOnce sets at-least-once delivery (JetStream).
func WithAtLeastOnce() Option {
	return func(cfg *TopicConfig) {
		cfg.DeliveryGuarantee = AtLeastOnce
	}
}

// WithAtMostOnce sets at-most-once delivery (core NATS).
func WithAtMostOnce() Option {
	return func(cfg *TopicConfig) {
		cfg.DeliveryGuarantee = AtMostOnce
	}
}

// WithStreamConfig overrides the StreamConfig template.
//
// Name/Subjects from this config are used unless overridden by WithStreamName/WithStreamSubjects.
func WithStreamConfig(sc nats.StreamConfig) Option {
	return func(cfg *TopicConfig) { cfg.Stream = sc }
}

// WithStreamName sets an explicit stream name.
func WithStreamName(name string) Option {
	return func(cfg *TopicConfig) { cfg.StreamName = name }
}

// WithStreamSubjects sets explicit stream subjects.
func WithStreamSubjects(subjects ...string) Option {
	return func(cfg *TopicConfig) {
		cfg.StreamSubjects = append([]string(nil), subjects...)
	}
}

// WithSubscriptionOptions sets AckWait, MaxInflight, QueueGroup.
func WithSubscriptionOptions(ackWait time.Duration, maxInflight int, queue string) Option {
	return func(cfg *TopicConfig) {
		cfg.AckWait = ackWait
		cfg.MaxInflight = maxInflight
		cfg.QueueGroup = queue
	}
}

// ensureStream idempotently creates/verifies the JetStream stream.
func (c *Client) ensureStream(name string, sc nats.StreamConfig) error {
	c.setupMutex.Lock()
	defer c.setupMutex.Unlock()

	if _, exists := c.streams[name]; exists {
		return nil
	}

	if info, err := c.js.StreamInfo(name); err == nil {
		if !subjectsCover(info.Config.Subjects, sc.Subjects) {
			return fmt.Errorf("existing stream %q subjects %v do not cover requested subjects %v", name, info.Config.Subjects, sc.Subjects)
		}
		c.streams[name] = struct{}{}
		return nil
	} else if !errors.Is(err, nats.ErrStreamNotFound) {
		return err
	}

	if _, err := c.js.AddStream(&sc); err != nil {
		if errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
			info, infoErr := c.js.StreamInfo(name)
			if infoErr != nil {
				return infoErr
			}
			if !subjectsCover(info.Config.Subjects, sc.Subjects) {
				return fmt.Errorf("stream %q already exists with incompatible subjects %v (need %v)", name, info.Config.Subjects, sc.Subjects)
			}
		} else {
			return err
		}
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

	if t.cfg.DeliveryGuarantee == AtLeastOnce {
		sc := t.streamConfig()
		if err := t.client.ensureStream(sc.Name, sc); err != nil {
			t.client.metrics.errorCounter.WithLabelValues(t.subject, "stream_setup").Inc()
			return "", err
		}

		ack, err := t.client.js.Publish(t.subject, data, nats.Context(ctx))
		if err != nil {
			t.client.metrics.errorCounter.WithLabelValues(t.subject, "publish").Inc()
			return "", err
		}

		t.client.metrics.publishCounter.WithLabelValues(t.subject).Inc()
		return fmt.Sprint(ack.Sequence), nil
	}

	if err := t.client.nc.Publish(t.subject, data); err != nil {
		t.client.metrics.errorCounter.WithLabelValues(t.subject, "publish").Inc()
		return "", err
	}

	t.client.metrics.publishCounter.WithLabelValues(t.subject).Inc()
	return "", nil
}

// SubscriptionConfig[T] holds your handler for T.
type SubscriptionConfig[T any] struct {
	Handler func(context.Context, *T) error
}

// Subscribe starts consuming events.
//
//   - AtLeastOnce: JetStream subscription with manual acking.
//   - AtMostOnce: core NATS subscription.
func (t *Topic[T]) Subscribe(durable string, cfg SubscriptionConfig[T]) error {
	handler := func(msg *nats.Msg) {
		ctx, span := t.client.tracer.Start(
			context.Background(),
			"HandleMessage",
			trace.WithAttributes(attribute.String("subject", t.subject)),
		)
		defer span.End()

		isJS := isJetStreamMessage(msg)

		var e T
		if err := json.Unmarshal(msg.Data, &e); err != nil {
			t.client.metrics.errorCounter.WithLabelValues(t.subject, "unmarshal").Inc()
			if isJS {
				_ = msg.Term()
			}
			return
		}

		if err := cfg.Handler(ctx, &e); err != nil {
			t.client.metrics.errorCounter.WithLabelValues(t.subject, "handler").Inc()
			if isJS {
				_ = msg.Nak()
			}
			return
		}

		t.client.metrics.subscribeCounter.WithLabelValues(t.subject).Inc()
		if isJS {
			_ = msg.Ack()
		}
	}

	if t.cfg.DeliveryGuarantee == AtLeastOnce {
		sc := t.streamConfig()
		if err := t.client.ensureStream(sc.Name, sc); err != nil {
			return err
		}

		subOpts := []nats.SubOpt{
			nats.ManualAck(),
			nats.AckWait(t.cfg.AckWait),
			nats.MaxAckPending(t.cfg.MaxInflight),
		}
		if durable != "" {
			subOpts = append(subOpts, nats.Durable(durable))
		}

		if t.cfg.QueueGroup != "" {
			_, err := t.client.js.QueueSubscribe(t.subject, t.cfg.QueueGroup, handler, subOpts...)
			return err
		}
		_, err := t.client.js.Subscribe(t.subject, handler, subOpts...)
		return err
	}

	if t.cfg.QueueGroup != "" {
		_, err := t.client.nc.QueueSubscribe(t.subject, t.cfg.QueueGroup, handler)
		return err
	}
	_, err := t.client.nc.Subscribe(t.subject, handler)
	return err
}

// PartitionedTopic publishes/consumes a base subject with user-scoped subject suffixes.
type PartitionedTopic[T any] struct{ topic *Topic[T] }

// NewPartitionedTopic[T] constructs a partitioned topic.
func NewPartitionedTopic[T any](c *Client, subject string, opts ...Option) *PartitionedTopic[T] {
	topic := NewTopic[T](c, subject, opts...)
	topic.ensureWildcardCoverage(subject)
	return &PartitionedTopic[T]{topic: topic}
}

// PublishForUser sends to "<subject>.<userID>".
func (pt *PartitionedTopic[T]) PublishForUser(ctx context.Context, userID string, evt *T) (string, error) {
	if userID == "" {
		return "", errors.New("userID cannot be empty")
	}
	topic := *pt.topic
	topic.subject = fmt.Sprintf("%s.%s", pt.topic.subject, userID)
	return topic.Publish(ctx, evt)
}

// BucketedTopic publishes/consumes on modulo-N subject partitions.
type BucketedTopic[T any] struct {
	pt         *PartitionedTopic[T]
	partitions int
}

// NewBucketedTopic[T] constructs a bucketed topic.
//
// partitions <= 0 is coerced to 1 to avoid runtime panics.
func NewBucketedTopic[T any](c *Client, subject string, partitions int, opts ...Option) *BucketedTopic[T] {
	if partitions <= 0 {
		partitions = 1
	}
	return &BucketedTopic[T]{
		pt:         NewPartitionedTopic[T](c, subject, opts...),
		partitions: partitions,
	}
}

// PublishWithKey hashes key and routes to key%N partition.
func (bt *BucketedTopic[T]) PublishWithKey(ctx context.Context, key string, evt *T) (string, error) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	idx := int(h.Sum32()) % bt.partitions

	topic := *bt.pt.topic
	topic.subject = fmt.Sprintf("%s.%d", bt.pt.topic.subject, idx)
	return topic.Publish(ctx, evt)
}

// SubscribePartition binds to a single partition.
func (bt *BucketedTopic[T]) SubscribePartition(durable string, partition int, cfg SubscriptionConfig[T]) error {
	topic := *bt.pt.topic
	topic.subject = fmt.Sprintf("%s.%d", bt.pt.topic.subject, partition)
	return topic.Subscribe(durable, cfg)
}

func (t *Topic[T]) streamConfig() nats.StreamConfig {
	sc := t.cfg.Stream
	sc.Name = t.streamName
	sc.Subjects = append([]string(nil), t.streamSubjects...)
	if sc.MaxAge == 0 {
		sc.MaxAge = defaultStreamTTL
	}
	if sc.Replicas == 0 {
		sc.Replicas = 1
	}
	if sc.Storage == 0 {
		sc.Storage = nats.FileStorage
	}
	return sc
}

func (t *Topic[T]) ensureWildcardCoverage(baseSubject string) {
	wildcard := baseSubject + ".>"
	if len(t.streamSubjects) == 0 {
		t.streamSubjects = []string{wildcard}
		return
	}
	for _, s := range t.streamSubjects {
		if s == wildcard {
			return
		}
		if strings.HasPrefix(s, baseSubject+".") && (strings.Contains(s, "*") || strings.Contains(s, ">")) {
			return
		}
	}
	t.streamSubjects = append(t.streamSubjects, wildcard)
}

func defaultStreamName(subject string) string {
	s := strings.TrimSpace(subject)
	if s == "" {
		return "encore_pubsub"
	}
	s = streamNameSanitizer.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		s = "encore_pubsub"
	}
	// Keep stream names short/stable for operational friendliness.
	const max = 200
	if len(s) > max {
		s = s[:max]
	}
	return "encore_pubsub_" + s
}

func subjectsCover(existing []string, required []string) bool {
	for _, need := range required {
		covered := false
		for _, have := range existing {
			if subjectPatternMatches(have, need) {
				covered = true
				break
			}
		}
		if !covered {
			return false
		}
	}
	return true
}

func subjectPatternMatches(pattern string, subject string) bool {
	p := strings.Split(pattern, ".")
	s := strings.Split(subject, ".")

	for i := 0; i < len(p); i++ {
		if i >= len(s) {
			return p[i] == ">" && i == len(p)-1
		}

		switch p[i] {
		case ">":
			return i == len(p)-1
		case "*":
			continue
		default:
			if p[i] != s[i] {
				return false
			}
		}
	}

	return len(s) == len(p)
}

func isJetStreamMessage(msg *nats.Msg) bool {
	if msg == nil {
		return false
	}
	_, err := msg.Metadata()
	return err == nil
}

func newMetrics() *metrics {
	m := &metrics{
		publishCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "pubsub_publish_total", Help: "Total number of published Pub/Sub events."},
			[]string{"subject"},
		),
		subscribeCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "pubsub_subscribe_total", Help: "Total number of consumed Pub/Sub events."},
			[]string{"subject"},
		),
		errorCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "pubsub_errors_total", Help: "Total number of Pub/Sub errors by phase."},
			[]string{"subject", "phase"},
		),
	}

	m.publishCounter = registerCounterVec(m.publishCounter)
	m.subscribeCounter = registerCounterVec(m.subscribeCounter)
	m.errorCounter = registerCounterVec(m.errorCounter)
	return m
}

func registerCounterVec(cv *prometheus.CounterVec) *prometheus.CounterVec {
	if err := prometheus.Register(cv); err != nil {
		var already prometheus.AlreadyRegisteredError
		if errors.As(err, &already) {
			if existing, ok := already.ExistingCollector.(*prometheus.CounterVec); ok {
				return existing
			}
		}
	}
	return cv
}
