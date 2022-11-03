package nsq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nsqio/go-nsq"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/reqtrack"
	"encore.dev/beta/errs"
	"encore.dev/pubsub/internal/types"
	"encore.dev/pubsub/internal/utils"
)

type Manager struct {
	ctx context.Context
	cfg *config.Config
	rt  *reqtrack.RequestTracker
}

func NewManager(ctx context.Context, cfg *config.Config, rt *reqtrack.RequestTracker) *Manager {
	return &Manager{ctx, cfg, rt}
}

// topic is the nsq implementation of pubsub.Topic. It exposes methods to publish
// and subscribe to messages of a topic
type topic struct {
	mgr       *Manager
	name      string
	addr      string
	m         sync.Mutex
	producer  *nsq.Producer
	consumers map[string]*nsq.Consumer
	idSeq     uint32
}

func (mgr *Manager) NewTopic(server *config.NSQProvider, topicCfg *config.PubsubTopic) types.TopicImplementation {
	return &topic{
		mgr:       mgr,
		name:      topicCfg.EncoreName,
		addr:      server.Host,
		producer:  nil,
		consumers: make(map[string]*nsq.Consumer),
		idSeq:     0,
	}
}

// messageWrapper is a local representation of a topic published to NSQ.
// it wraps the raw data with an ID and an Attribute map.
// It must be synchronized with the e2e-tests/testscript_test.go file.
type messageWrapper struct {
	ID         string
	Attributes map[string]string
	Data       json.RawMessage
}

func (l *topic) Subscribe(logger *zerolog.Logger, ackDeadline time.Duration, retryPolicy *types.RetryPolicy, implCfg *config.PubsubSubscription, f types.RawSubscriptionCallback) {
	if implCfg.PushOnly {
		panic("push-only subscriptions are not supported by nsq")
	}

	l.m.Lock()
	defer l.m.Unlock()

	if _, ok := l.consumers[implCfg.EncoreName]; ok {
		panic("NewSubscription must use a unique subscription name")
	}
	conCfg := nsq.NewConfig()
	consumer, err := nsq.NewConsumer(l.name, implCfg.EncoreName, conCfg)
	if err != nil {
		panic(fmt.Sprintf("unable to setup subscription %s for topic %s: %v", implCfg.EncoreName, l.name, err))
	}
	// only log warnings and above from the NSQ library
	consumer.SetLogger(&LogAdapter{Logger: logger}, nsq.LogLevelWarning)

	// create a dedicated handler which forwards messages to the encore subscription
	consumer.AddHandler(nsq.HandlerFunc(func(m *nsq.Message) error {
		// create a message to unmarshal the raw nsq body into
		msg := &messageWrapper{}

		defer func() {
			if !m.HasResponded() {
				retry, delay := utils.GetDelay(retryPolicy.MaxRetries, retryPolicy.MinBackoff, retryPolicy.MaxBackoff, m.Attempts)
				if !retry {

					logger.Error().Str("msg_id", msg.ID).Int("retry", int(m.Attempts)-1).Msg("depleted message retries. Dropping message")
					// TODO; offload this to the dead letter queue
					m.Finish()
					return
				}
				m.RequeueWithoutBackoff(delay)
			}
		}()

		err = json.Unmarshal(m.Body, msg)
		if err != nil {
			return errs.B().Cause(err).Code(errs.InvalidArgument).Msg("failed to unmarshal message wrapper").Err()
		}

		// forward the message to the subscriber
		msgCtx, cancel := context.WithTimeout(l.mgr.ctx, ackDeadline)
		defer cancel()

		err = f(msgCtx, msg.ID, time.Unix(0, m.Timestamp), int(m.Attempts), msg.Attributes, msg.Data)
		if err != nil {
			return err
		}
		m.Finish()
		return nil
	}))

	// connect the consumer to the NSQD
	err = consumer.ConnectToNSQD(l.addr)
	if err != nil {
		panic(fmt.Sprintf("failed to connect %s to nsqd for topic %s: %v", implCfg.EncoreName, l.name, err))
	}
	// add the consumer to the known consumers
	l.consumers[implCfg.EncoreName] = consumer
}

// PublishMessage publishes a message to an nsq Topic
func (l *topic) PublishMessage(_ context.Context, attrs map[string]string, data []byte) (id string, err error) {
	// instantiate a Producer if there isn;t one already
	if l.producer == nil {
		l.m.Lock()
		defer l.m.Unlock()
		if l.producer == nil {
			cfg := nsq.NewConfig()
			producer, err := nsq.NewProducer(l.addr, cfg)
			if err != nil {
				return "", errs.B().Cause(err).Code(errs.Internal).Msg("failed to connect to NSQD").Err()
			}
			// only log warnings and above from the NSQ library
			log := l.mgr.rt.Logger().With().Str("topic", l.name).Logger()
			producer.SetLogger(&LogAdapter{Logger: &log}, nsq.LogLevelWarning)
			l.producer = producer
		}
	}
	// generate a new message ID
	idx := fmt.Sprint(atomic.AddUint32(&l.idSeq, 1))

	// create and publish the message wrapper
	data, err = json.Marshal(&messageWrapper{ID: idx, Data: data, Attributes: attrs})
	if err != nil {
		return "", errs.B().Cause(err).Code(errs.Internal).Msg("failed to marshal message").Err()
	}
	err = l.producer.Publish(l.name, data)
	if err != nil {
		return "", errs.B().Cause(err).Code(errs.Internal).Msg("failed to connect to NSQD").Err()
	}
	// return the message id!
	return idx, nil
}
