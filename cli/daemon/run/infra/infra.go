package infra

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encore.dev/appruntime/exported/config"
	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/cli/daemon/pubsub"
	"encr.dev/cli/daemon/redis"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/environ"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

type Type string

const (
	PubSub Type = "pubsub"
	Cache  Type = "cache"
	SQLDB  Type = "sqldb"
)

const (
	// this ID is used in the Encore Cloud README file as an example
	// on how to create a topic resource
	encoreCloudExampleTopicID = "res_0o9ioqnrirflhhm3t720"

	// this ID is used in the Encore Cloud README file as a example
	// on how to create a subscription on the above topic
	encoreCloudExampleSubscriptionID = "res_0o9ioqnrirflhhm3t730"
)

// ResourceManager manages a set of infrastructure resources
// to support the running Encore application.
type ResourceManager struct {
	app         *apps.Instance
	dbProxyPort int
	sqlMgr      *sqldb.ClusterManager
	ns          *namespace.Namespace
	environ     environ.Environ
	log         zerolog.Logger
	forTests    bool

	mutex   sync.Mutex
	servers map[Type]Resource
}

func NewResourceManager(app *apps.Instance, sqlMgr *sqldb.ClusterManager, ns *namespace.Namespace, environ environ.Environ, dbProxyPort int, forTests bool) *ResourceManager {
	return &ResourceManager{
		app:         app,
		dbProxyPort: dbProxyPort,
		sqlMgr:      sqlMgr,
		ns:          ns,
		environ:     environ,
		forTests:    forTests,

		servers: make(map[Type]Resource),
		log:     log.With().Str("app_id", app.PlatformOrLocalID()).Logger(),
	}
}

func (rm *ResourceManager) StopAll() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rm.log.Info().Int("num", len(rm.servers)).Msg("Stopping all resource services")

	for _, daemon := range rm.servers {
		daemon.Stop()
	}
}

type Resource interface {
	// Stop shuts down the resource.
	Stop()
}

// StartRequiredServices will start the required services for the current application
// if they are not already running based on the given parse result
func (rm *ResourceManager) StartRequiredServices(a *optracker.AsyncBuildJobs, md *meta.Data) {
	if sqldb.IsUsed(md) && rm.GetSQLCluster() == nil {
		a.Go("Creating PostgreSQL database cluster", true, 300*time.Millisecond, rm.StartSQLCluster(a, md))
	}

	if pubsub.IsUsed(md) && rm.GetPubSub() == nil {
		a.Go("Starting PubSub daemon", true, 250*time.Millisecond, rm.StartPubSub)
	}

	if redis.IsUsed(md) && rm.GetRedis() == nil {
		a.Go("Starting Redis server", true, 250*time.Millisecond, rm.StartRedis)
	}
}

// StartPubSub starts a PubSub daemon.
func (rm *ResourceManager) StartPubSub(ctx context.Context) error {
	nsqd := &pubsub.NSQDaemon{}
	err := nsqd.Start()
	if err != nil {
		return err
	}

	rm.mutex.Lock()
	rm.servers[PubSub] = nsqd
	rm.mutex.Unlock()
	return nil
}

// GetPubSub returns the PubSub daemon if it is running otherwise it returns nil
func (rm *ResourceManager) GetPubSub() *pubsub.NSQDaemon {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if daemon, found := rm.servers[PubSub]; found {
		return daemon.(*pubsub.NSQDaemon)
	}
	return nil
}

// StartRedis starts a Redis server.
func (rm *ResourceManager) StartRedis(ctx context.Context) error {
	srv := redis.New()
	err := srv.Start()
	if err != nil {
		return err
	}

	rm.mutex.Lock()
	rm.servers[Cache] = srv
	rm.mutex.Unlock()
	return nil
}

// GetRedis returns the Redis server if it is running otherwise it returns nil
func (rm *ResourceManager) GetRedis() *redis.Server {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if srv, found := rm.servers[Cache]; found {
		return srv.(*redis.Server)
	}
	return nil
}

func (rm *ResourceManager) StartSQLCluster(a *optracker.AsyncBuildJobs, md *meta.Data) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// This can be the case in tests.
		if rm.sqlMgr == nil {
			return fmt.Errorf("StartSQLCluster: no SQL Cluster manager provided")
		}

		typ := sqldb.Run
		if rm.forTests {
			typ = sqldb.Test
		}

		if err := rm.sqlMgr.Ready(); err != nil {
			return err
		}

		cluster := rm.sqlMgr.Create(ctx, &sqldb.CreateParams{
			ClusterID: sqldb.GetClusterID(rm.app, typ, rm.ns),
			Memfs:     rm.forTests,
		})

		if _, err := cluster.Start(ctx, a.Tracker()); err != nil {
			return fmt.Errorf("unable to start sqldb cluster: %w", err)
		}

		rm.mutex.Lock()
		rm.servers[SQLDB] = cluster
		rm.mutex.Unlock()

		// Set up the database asynchronously since it can take a while.
		if rm.forTests {
			a.Go("Recreating databases", true, 250*time.Millisecond, func(ctx context.Context) error {
				err := cluster.Recreate(ctx, rm.app.Root(), nil, md)
				if err != nil {
					rm.log.Error().Err(err).Msg("failed to recreate db")
					return err
				}
				return nil
			})
		} else {
			a.Go("Running database migrations", true, 250*time.Millisecond, func(ctx context.Context) error {
				err := cluster.SetupAndMigrate(ctx, rm.app.Root(), md)
				if err != nil {
					rm.log.Error().Err(err).Msg("failed to setup db")
					return err
				}
				return nil
			})
		}

		return nil
	}
}

// GetSQLCluster returns the SQL cluster
func (rm *ResourceManager) GetSQLCluster() *sqldb.Cluster {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if cluster, found := rm.servers[SQLDB]; found {
		return cluster.(*sqldb.Cluster)
	}
	return nil
}

// UpdateConfig updates the given config with infrastructure information.
// Note that all the requisite services must have started up already,
// which in practice means that (*optracker.AsyncBuildJobs).Wait must have returned first.
func (rm *ResourceManager) UpdateConfig(cfg *config.Runtime, md *meta.Data, dbProxyPort int) error {
	useLocalEncoreCloudAPIForTesting, err := rm.setTestEncoreCloud(cfg)
	if err != nil {
		return err
	}

	if cluster := rm.GetSQLCluster(); cluster != nil {
		srv := &config.SQLServer{
			Host: "localhost:" + strconv.Itoa(dbProxyPort),
		}
		serverID := len(cfg.SQLServers)
		cfg.SQLServers = append(cfg.SQLServers, srv)

		for _, db := range md.SqlDatabases {
			cfg.SQLDatabases = append(cfg.SQLDatabases, &config.SQLDatabase{
				ServerID:     serverID,
				EncoreName:   db.Name,
				DatabaseName: db.Name,
				User:         "encore",
				Password:     cluster.Password,
			})
		}

		// Configure max connections based on 96 connections
		// divided evenly among the databases
		maxConns := 96 / len(cfg.SQLDatabases)
		for _, db := range cfg.SQLDatabases {
			db.MaxConnections = maxConns
		}
	}

	if nsq := rm.GetPubSub(); nsq != nil {
		provider := &config.PubsubProvider{
			NSQ: &config.NSQProvider{
				Host: nsq.Addr(),
			},
		}
		providerID := len(cfg.PubsubProviders)
		cfg.PubsubProviders = append(cfg.PubsubProviders, provider)

		// If we're testing the Encore Cloud API locally, override from NSQ
		if useLocalEncoreCloudAPIForTesting {
			providerID = len(cfg.PubsubProviders)
			cfg.PubsubProviders = append(cfg.PubsubProviders, &config.PubsubProvider{
				EncoreCloud: &config.EncoreCloudPubsubProvider{},
			})
		}

		cfg.PubsubTopics = make(map[string]*config.PubsubTopic)
		for _, t := range md.PubsubTopics {
			providerName := t.Name
			if useLocalEncoreCloudAPIForTesting {
				providerName = encoreCloudExampleTopicID
			}

			topicCfg := &config.PubsubTopic{
				ProviderID:    providerID,
				EncoreName:    t.Name,
				ProviderName:  providerName,
				Subscriptions: make(map[string]*config.PubsubSubscription),
			}

			for _, s := range t.Subscriptions {
				subscriptionID := t.Name
				if useLocalEncoreCloudAPIForTesting {
					subscriptionID = encoreCloudExampleSubscriptionID
				}

				topicCfg.Subscriptions[s.Name] = &config.PubsubSubscription{
					ID:           subscriptionID,
					EncoreName:   s.Name,
					ProviderName: s.Name,
				}
			}

			cfg.PubsubTopics[t.Name] = topicCfg
		}
	}

	if redis := rm.GetRedis(); redis != nil {
		srv := &config.RedisServer{
			Host: redis.Addr(),
		}
		serverID := len(cfg.RedisServers)
		cfg.RedisServers = append(cfg.RedisServers, srv)

		for _, cluster := range md.CacheClusters {
			cfg.RedisDatabases = append(cfg.RedisDatabases, &config.RedisDatabase{
				ServerID:   serverID,
				Database:   0,
				EncoreName: cluster.Name,
				KeyPrefix:  cluster.Name + "/",
			})
		}
	}

	return nil
}

// SQLConfig returns the SQL server and database configuration for the given database.
func (rm *ResourceManager) SQLConfig(db *meta.SQLDatabase) (config.SQLServer, config.SQLDatabase, error) {
	cluster := rm.GetSQLCluster()
	if cluster == nil {
		return config.SQLServer{}, config.SQLDatabase{}, errors.New("no SQL cluster found")
	}

	srvCfg := config.SQLServer{
		Host: "localhost:" + strconv.Itoa(rm.dbProxyPort),
	}

	dbCfg := config.SQLDatabase{
		EncoreName:   db.Name,
		DatabaseName: db.Name,
		User:         "encore",
		Password:     cluster.Password,
	}

	return srvCfg, dbCfg, nil
}

// PubSubTopicConfig returns the PubSub provider and topic configuration for the given topic.
func (rm *ResourceManager) PubSubTopicConfig(topic *meta.PubSubTopic) (config.PubsubProvider, config.PubsubTopic, error) {
	nsq := rm.GetPubSub()
	if nsq == nil {
		return config.PubsubProvider{}, config.PubsubTopic{}, errors.New("no PubSub server found")
	}

	providerCfg := config.PubsubProvider{
		NSQ: &config.NSQProvider{
			Host: nsq.Addr(),
		},
	}

	topicCfg := config.PubsubTopic{
		EncoreName:    topic.Name,
		ProviderName:  topic.Name,
		Subscriptions: make(map[string]*config.PubsubSubscription),
	}

	return providerCfg, topicCfg, nil
}

// PubSubSubscriptionConfig returns the PubSub subscription configuration for the given subscription.
func (rm *ResourceManager) PubSubSubscriptionConfig(_ *meta.PubSubTopic, sub *meta.PubSubTopic_Subscription) (config.PubsubSubscription, error) {
	subCfg := config.PubsubSubscription{
		ID:           sub.Name,
		EncoreName:   sub.Name,
		ProviderName: sub.Name,
	}

	return subCfg, nil
}

// RedisConfig returns the Redis server and database configuration for the given database.
func (rm *ResourceManager) RedisConfig(redis *meta.CacheCluster) (config.RedisServer, config.RedisDatabase, error) {
	server := rm.GetRedis()
	if server == nil {
		return config.RedisServer{}, config.RedisDatabase{}, errors.New("no Redis server found")
	}

	srvCfg := config.RedisServer{
		Host: server.Addr(),
	}

	dbCfg := config.RedisDatabase{
		EncoreName: redis.Name,
		KeyPrefix:  redis.Name + "/",
	}

	return srvCfg, dbCfg, nil
}
