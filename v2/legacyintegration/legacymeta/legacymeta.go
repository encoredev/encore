package legacymeta

import (
	"fmt"

	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/infra/resource/cache"
	"encr.dev/v2/parser/infra/resource/cron"
	"encr.dev/v2/parser/infra/resource/metrics"
	"encr.dev/v2/parser/infra/resource/pubsub"
)

type builder struct {
	errs    *perr.List
	svcName string
	res     parser.Result
	md      *meta.Data // metadata being generated
}

func Gen(errs *perr.List, res parser.Result, serviceName string) *meta.Data {
	b := &builder{
		errs:    errs,
		svcName: serviceName,
		res:     res,
	}
	return b.Build()
}

func (b *builder) Build() *meta.Data {
	b.md = &meta.Data{
		ModulePath:  "example.com", // TODO
		AppRevision: "123",         // TODO
	}
	md := b.md

	svc := &meta.Service{
		Name:       b.svcName,
		RelPath:    ".",
		Rpcs:       nil,
		Migrations: nil, // TODO
		Databases:  nil, // TODO
		HasConfig:  false,
	}
	md.Svcs = append(md.Svcs, svc)

	// TODO
	var (
		subscriptions []*meta.PubSubTopic_Subscription
		keyspaces     []*meta.CacheCluster_Keyspace
	)

	for _, r := range b.res.Resources {
		switch r := r.(type) {
		case *cron.Job:
			md.CronJobs = append(md.CronJobs, &meta.CronJob{
				Id:       r.Name,
				Title:    r.Title,
				Doc:      r.Doc,
				Schedule: r.Schedule,
				Endpoint: nil, // TODO
			})

		case *pubsub.Topic:
			topic := &meta.PubSubTopic{
				Name:          r.Name,
				Doc:           r.Doc,
				MessageType:   b.typeDeclRef(r.MessageType),
				OrderingKey:   r.OrderingKey,
				Publishers:    nil, // TODO
				Subscriptions: nil, // TODO
			}

			switch r.DeliveryGuarantee {
			case pubsub.ExactlyOnce:
				topic.DeliveryGuarantee = meta.PubSubTopic_EXACTLY_ONCE
			case pubsub.AtLeastOnce:
				topic.DeliveryGuarantee = meta.PubSubTopic_AT_LEAST_ONCE
			default:
				panic(fmt.Sprintf("unknown delivery guarantee %v", r.DeliveryGuarantee))
			}

			md.PubsubTopics = append(md.PubsubTopics, topic)

		case *pubsub.Subscription:
			sub := &meta.PubSubTopic_Subscription{
				Name:             r.Name,
				ServiceName:      b.svcName,
				AckDeadline:      0,   // TODO
				MessageRetention: 0,   // TODO
				RetryPolicy:      nil, // TODO
			}
			subscriptions = append(subscriptions, sub)

		case *cache.Cluster:
			md.CacheClusters = append(md.CacheClusters, &meta.CacheCluster{
				Name:           r.Name,
				Doc:            r.Doc,
				Keyspaces:      nil,
				EvictionPolicy: r.EvictionPolicy,
			})

		case *cache.Keyspace:
			ks := &meta.CacheCluster_Keyspace{
				KeyType:     b.schemaType(r.KeyType),
				ValueType:   b.schemaType(r.ValueType),
				Service:     "",
				Doc:         r.Doc,
				PathPattern: b.keyspacePath(r.Path),
			}
			keyspaces = append(keyspaces, ks)

		case *metrics.Metric:
			m := &meta.Metric{
				Name:        r.Name,
				ValueType:   b.builtinType(r.ValueType),
				Doc:         r.Doc,
				ServiceName: &b.svcName,
				Labels:      nil, // TODO
			}
			switch r.Type {
			case metrics.Counter:
				m.Kind = meta.Metric_COUNTER
			case metrics.Gauge:
				m.Kind = meta.Metric_GAUGE
			default:
				panic(fmt.Sprintf("unknown metric type %v", r.Type))
			}

			md.Metrics = append(md.Metrics, m)
		}
	}

	return md
}

func (b *builder) keyspacePath(path *cache.KeyspacePath) *meta.Path {
	res := &meta.Path{
		Type: meta.Path_CACHE_KEYSPACE,
	}
	for _, p := range path.Segments {
		seg := &meta.PathSegment{
			Value: p.Value,
		}

		switch p.Type {
		case cache.Param:
			seg.Type = meta.PathSegment_PARAM
		case cache.Literal:
			seg.Type = meta.PathSegment_LITERAL
		}

		res.Segments = append(res.Segments, seg)
	}
	return res
}
