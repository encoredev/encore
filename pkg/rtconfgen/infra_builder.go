package rtconfgen

import (
	"slices"

	meta "encr.dev/proto/encore/parser/meta/v1"
	runtimev1 "encr.dev/proto/encore/runtime/v1"
)

type InfraBuilder struct {
	infra *runtimev1.Infrastructure
	rs    *resourceSet
}

func newInfraBuilder(rs *resourceSet) *InfraBuilder {
	infra := &runtimev1.Infrastructure{
		Credentials: &runtimev1.Infrastructure_Credentials{},
		Resources:   &runtimev1.Infrastructure_Resources{},
	}
	return &InfraBuilder{
		infra: infra,
		rs:    rs,
	}
}

func (b *InfraBuilder) ClientCert(rid string, fn func() *runtimev1.ClientCert) *ClientCert {
	val := addResFunc(&b.infra.Credentials.ClientCerts, b.rs, rid, fn)
	return &ClientCert{Val: val, b: b}
}

type ClientCert struct {
	Val *runtimev1.ClientCert
	b   *InfraBuilder
}

func (b *InfraBuilder) SQLRole(p *runtimev1.SQLRole) *SQLRole {
	return b.SQLRoleFn(p.Rid, tofn(p))
}

func (b *InfraBuilder) SQLRoleFn(rid string, fn func() *runtimev1.SQLRole) *SQLRole {
	val := addResFunc(&b.infra.Credentials.SqlRoles, b.rs, rid, fn)
	return &SQLRole{Val: val, b: b}
}

type SQLRole struct {
	Val *runtimev1.SQLRole
	b   *InfraBuilder
}

func (b *InfraBuilder) SQLCluster(p *runtimev1.SQLCluster) *SQLCluster {
	return b.SQLClusterFn(p.Rid, tofn(p))
}

func (b *InfraBuilder) SQLClusterFn(rid string, fn func() *runtimev1.SQLCluster) *SQLCluster {
	val := addResFunc(&b.infra.Resources.SqlClusters, b.rs, rid, fn)
	return &SQLCluster{Val: val, b: b}
}

type SQLCluster struct {
	Val *runtimev1.SQLCluster
	b   *InfraBuilder
}

func (c *SQLCluster) SQLDatabase(p *runtimev1.SQLDatabase) *SQLDatabase {
	return c.SQLDatabaseFn(p.Rid, tofn(p))
}

func (c *SQLCluster) SQLDatabaseFn(rid string, fn func() *runtimev1.SQLDatabase) *SQLDatabase {
	val := addResFunc(&c.Val.Databases, c.b.rs, rid, fn)
	return &SQLDatabase{Val: val, b: c.b}
}

type SQLDatabase struct {
	Val *runtimev1.SQLDatabase
	b   *InfraBuilder
}

func (c *SQLDatabase) AddConnectionPool(p *runtimev1.SQLConnectionPool) {
	c.Val.ConnPools = append(c.Val.ConnPools, p)
}

func (c *SQLCluster) SQLServer(p *runtimev1.SQLServer) *SQLServer {
	return c.SQLServerFn(p.Rid, tofn(p))
}

func (c *SQLCluster) SQLServerFn(rid string, fn func() *runtimev1.SQLServer) *SQLServer {
	val := addResFunc(&c.Val.Servers, c.b.rs, rid, fn)
	return &SQLServer{Val: val, b: c.b}
}

type SQLServer struct {
	Val *runtimev1.SQLServer
	b   *InfraBuilder
}

func (b *InfraBuilder) PubSubCluster(p *runtimev1.PubSubCluster) *PubSubCluster {
	return b.PubSubClusterFn(p.Rid, tofn(p))
}

func (b *InfraBuilder) PubSubClusterFn(rid string, fn func() *runtimev1.PubSubCluster) *PubSubCluster {
	val := addResFunc(&b.infra.Resources.PubsubClusters, b.rs, rid, fn)
	return &PubSubCluster{Val: val, b: b}
}

type PubSubCluster struct {
	Val *runtimev1.PubSubCluster
	b   *InfraBuilder
}

func (c *PubSubCluster) PubSubTopic(p *runtimev1.PubSubTopic) *PubSubTopic {
	return c.PubSubTopicFn(p.Rid, tofn(p))
}

func (c *PubSubCluster) PubSubTopicFn(rid string, fn func() *runtimev1.PubSubTopic) *PubSubTopic {
	val := addResFunc(&c.Val.Topics, c.b.rs, rid, fn)
	return &PubSubTopic{Val: val, b: c.b}
}

type PubSubTopic struct {
	Val *runtimev1.PubSubTopic
	b   *InfraBuilder
}

func (c *PubSubCluster) PubSubSubscription(p *runtimev1.PubSubSubscription) *PubSubSubscription {
	return c.PubSubSubscriptionFn(p.Rid, tofn(p))
}

func (c *PubSubCluster) PubSubSubscriptionFn(rid string, fn func() *runtimev1.PubSubSubscription) *PubSubSubscription {
	val := addResFunc(&c.Val.Subscriptions, c.b.rs, rid, fn)
	return &PubSubSubscription{Val: val, b: c.b}
}

type PubSubSubscription struct {
	Val *runtimev1.PubSubSubscription
	b   *InfraBuilder
}

func (b *InfraBuilder) RedisRole(p *runtimev1.RedisRole) *RedisRole {
	return b.RedisRoleFn(p.Rid, tofn(p))
}

func (b *InfraBuilder) RedisRoleFn(rid string, fn func() *runtimev1.RedisRole) *RedisRole {
	val := addResFunc(&b.infra.Credentials.RedisRoles, b.rs, rid, fn)
	return &RedisRole{Val: val, b: b}
}

type RedisRole struct {
	Val *runtimev1.RedisRole
	b   *InfraBuilder
}

func (b *InfraBuilder) RedisCluster(p *runtimev1.RedisCluster) *RedisCluster {
	return b.RedisClusterFn(p.Rid, tofn(p))
}

func (b *InfraBuilder) RedisClusterFn(rid string, fn func() *runtimev1.RedisCluster) *RedisCluster {
	val := addResFunc(&b.infra.Resources.RedisClusters, b.rs, rid, fn)
	return &RedisCluster{Val: val, b: b}
}

type RedisCluster struct {
	Val *runtimev1.RedisCluster
	b   *InfraBuilder
}

func (c *RedisCluster) RedisDatabase(p *runtimev1.RedisDatabase) *RedisDatabase {
	return c.RedisDatabaseFn(p.Rid, tofn(p))
}

func (c *RedisCluster) RedisDatabaseFn(rid string, fn func() *runtimev1.RedisDatabase) *RedisDatabase {
	val := addResFunc(&c.Val.Databases, c.b.rs, rid, fn)
	return &RedisDatabase{Val: val, b: c.b}
}

type RedisDatabase struct {
	Val *runtimev1.RedisDatabase
	b   *InfraBuilder
}

func (c *RedisDatabase) AddConnectionPool(p *runtimev1.RedisConnectionPool) {
	c.Val.ConnPools = append(c.Val.ConnPools, p)
}

func (c *RedisCluster) RedisServer(p *runtimev1.RedisServer) *RedisServer {
	return c.RedisServerFn(p.Rid, tofn(p))
}

func (c *RedisCluster) RedisServerFn(rid string, fn func() *runtimev1.RedisServer) *RedisServer {
	val := addResFunc(&c.Val.Servers, c.b.rs, rid, fn)
	return &RedisServer{Val: val, b: c.b}
}

type RedisServer struct {
	Val *runtimev1.RedisServer
	b   *InfraBuilder
}

func (b *InfraBuilder) Gateway(gw *runtimev1.Gateway) *Gateway {
	return b.GatewayFn(gw.Rid, tofn(gw))
}

func (b *InfraBuilder) GatewayFn(rid string, fn func() *runtimev1.Gateway) *Gateway {
	val := addResFunc(&b.infra.Resources.Gateways, b.rs, rid, fn)
	return &Gateway{Val: val, b: b}
}

type Gateway struct {
	Val *runtimev1.Gateway
	b   *InfraBuilder
}

func (b *InfraBuilder) AppSecret(p *runtimev1.AppSecret) *AppSecret {
	return b.AppSecretFn(p.Rid, tofn(p))
}

func (b *InfraBuilder) AppSecretFn(rid string, fn func() *runtimev1.AppSecret) *AppSecret {
	val := addResFunc(&b.infra.Resources.AppSecrets, b.rs, rid, fn)
	return &AppSecret{Val: val, b: b}
}

type AppSecret struct {
	Val *runtimev1.AppSecret
	b   *InfraBuilder
}

func (b *InfraBuilder) get() (*runtimev1.Infrastructure, error) {
	return b.infra, nil
}

func tofn[V any](v V) func() V {
	return func() V { return v }
}

// reduceForServices reduces the given infrastructure to only include resource accessible by
// the given services, using the metadata for access control.
func reduceForServices(infra *runtimev1.Infrastructure, md *meta.Data, svcs []string) *runtimev1.Infrastructure {
	// Clone the protobuf so the changes don't affect the original.
	infra = cloneProto(infra)

	svcNames := make(map[string]bool)
	for _, svc := range svcs {
		svcNames[svc] = true
	}

	dbsToKeep := make(map[string]bool)
	for _, svc := range md.Svcs {
		if !svcNames[svc.Name] {
			continue
		}
		for _, dbName := range svc.Databases {
			dbsToKeep[dbName] = true
		}
	}

	type subKey struct {
		topicName string
		subName   string
	}
	topicsToKeep := make(map[string]bool)
	subsToKeep := make(map[subKey]bool)
	for _, topic := range md.PubsubTopics {
		for _, publisher := range topic.Publishers {
			if svcNames[publisher.ServiceName] {
				topicsToKeep[topic.Name] = true
			}
		}

		for _, subscriber := range topic.Subscriptions {
			if svcNames[subscriber.ServiceName] {
				subsToKeep[subKey{topicName: topic.Name, subName: subscriber.Name}] = true
			}
		}
	}

	cachesToKeep := make(map[string]bool)
	for _, cacheCluster := range md.CacheClusters {
		for _, keySpace := range cacheCluster.Keyspaces {
			if svcNames[keySpace.Service] {
				cachesToKeep[cacheCluster.Name] = true
			}
		}
	}

	for _, cluster := range infra.Resources.PubsubClusters {
		cluster.Topics = slices.DeleteFunc(cluster.Topics, func(t *runtimev1.PubSubTopic) bool {
			_, found := topicsToKeep[t.EncoreName]
			return !found
		})
		cluster.Subscriptions = slices.DeleteFunc(cluster.Subscriptions, func(t *runtimev1.PubSubSubscription) bool {
			_, found := subsToKeep[subKey{topicName: t.TopicEncoreName, subName: t.SubscriptionEncoreName}]
			return !found
		})
	}

	for _, cluster := range infra.Resources.RedisClusters {
		cluster.Databases = slices.DeleteFunc(cluster.Databases, func(t *runtimev1.RedisDatabase) bool {
			_, found := cachesToKeep[t.EncoreName]
			return !found
		})
	}

	secretsToKeep := secretsUsedByServices(md, svcNames)
	infra.Resources.AppSecrets = slices.DeleteFunc(infra.Resources.AppSecrets, func(t *runtimev1.AppSecret) bool {
		_, found := secretsToKeep[t.EncoreName]
		return !found
	})

	return infra
}

// secretsUsedByServices returns the set of secrets that are accessible by the given services, using the metadata for access control.
func secretsUsedByServices(md *meta.Data, svcNames map[string]bool) (secretNames map[string]bool) {
	secretNames = make(map[string]bool)
	for _, pkg := range md.Pkgs {
		if len(pkg.Secrets) > 0 && (pkg.ServiceName == "" || svcNames[pkg.ServiceName]) {
			for _, secret := range pkg.Secrets {
				secretNames[secret] = true
			}
		}
	}
	return secretNames
}
