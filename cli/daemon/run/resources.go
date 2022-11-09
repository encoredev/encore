package run

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/pubsub"
	"encr.dev/cli/daemon/redis"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/cli/daemon/sqldb/docker"
	"encr.dev/parser"
	"encr.dev/parser/est"
)

// ResourceServices represent the set of servers/services we have started up
// to support the running Encore application.
type ResourceServices struct {
	mutex   sync.Mutex
	servers map[est.ResourceType]ResourceServer

	app    *apps.Instance
	sqlMgr *sqldb.ClusterManager
	log    zerolog.Logger
}

func NewResourceServices(app *apps.Instance, sqlMgr *sqldb.ClusterManager) *ResourceServices {
	return &ResourceServices{
		app:    app,
		sqlMgr: sqlMgr,

		servers: make(map[est.ResourceType]ResourceServer),
		log:     log.With().Str("app_id", app.PlatformOrLocalID()).Logger(),
	}
}

func (rs *ResourceServices) StopAll() {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	rs.log.Info().Int("num", len(rs.servers)).Msg("Stopping all resource services")

	for _, daemon := range rs.servers {
		daemon.Stop()
	}
}

type ResourceServer interface {
	Stop() // Shutdown the resource
}

// StartRequiredServices will start the required services for the current application
// if they are not already running based on the given parse result
func (rs *ResourceServices) StartRequiredServices(a *AsyncBuildJobs, parse *parser.Result) error {
	if sqldb.IsUsed(parse.Meta) && rs.GetSQLCluster() == nil {
		a.Go("Creating PostgreSQL database cluster", true, 300*time.Millisecond, rs.StartSQLCluster(a, parse))
	}

	if pubsub.IsUsed(parse.Meta) && rs.GetPubSub() == nil {
		a.Go("Starting PubSub daemon", true, 250*time.Millisecond, rs.StartPubSub)
	}

	if redis.IsUsed(parse.Meta) && rs.GetRedis() == nil {
		a.Go("Starting Redis server", true, 250*time.Millisecond, rs.StartRedis)
	}

	return nil
}

// StartPubSub starts a PubSub daemon.
func (rs *ResourceServices) StartPubSub(ctx context.Context) error {
	nsqd := &pubsub.NSQDaemon{}
	err := nsqd.Start()
	if err != nil {
		return err
	}

	rs.mutex.Lock()
	rs.servers[est.PubSubTopicResource] = nsqd
	rs.mutex.Unlock()
	return nil
}

// GetPubSub returns the PubSub daemon if it is running otherwise it returns nil
func (rs *ResourceServices) GetPubSub() *pubsub.NSQDaemon {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if daemon, found := rs.servers[est.PubSubTopicResource]; found {
		return daemon.(*pubsub.NSQDaemon)
	}
	return nil
}

// StartRedis starts a Redis server.
func (rs *ResourceServices) StartRedis(ctx context.Context) error {
	srv := redis.New()
	err := srv.Start()
	if err != nil {
		return err
	}

	rs.mutex.Lock()
	rs.servers[est.CacheClusterResource] = srv
	rs.mutex.Unlock()
	return nil
}

// GetRedis returns the Redis server if it is running otherwise it returns nil
func (rs *ResourceServices) GetRedis() *redis.Server {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if srv, found := rs.servers[est.CacheClusterResource]; found {
		return srv.(*redis.Server)
	}
	return nil
}

func (rs *ResourceServices) StartSQLCluster(a *AsyncBuildJobs, parse *parser.Result) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// This can be the case in tests.
		if rs.sqlMgr == nil {
			return fmt.Errorf("StartSQLCluster: no SQL Cluster manager provided")
		}

		cluster := rs.sqlMgr.Create(ctx, &sqldb.CreateParams{
			ClusterID: sqldb.GetClusterID(rs.app, sqldb.Run),
			Memfs:     false,
		})
		if _, err := exec.LookPath("docker"); err != nil {
			return errors.New("This application requires docker to run since it uses an SQL database. Install docker first.")
		} else if !isDockerRunning(ctx) {
			return errors.New("The docker daemon is not running. Start it first.")
		}

		log.Debug().Msg("checking if sqldb image exists")
		if ok, err := docker.ImageExists(ctx); err == nil && !ok {
			rs.log.Debug().Msg("pulling sqldb image")
			pullOp := a.tracker.Add("Pulling PostgreSQL docker image", time.Now())
			if err := docker.PullImage(ctx); err != nil {
				rs.log.Error().Err(err).Msg("unable to pull sqldb image")
				a.tracker.Fail(pullOp, err)
			} else {
				a.tracker.Done(pullOp, 0)
				rs.log.Info().Msg("successfully pulled sqldb image")
			}
		} else if err != nil {
			return fmt.Errorf("unable to check if sqldb image exists: %w", err)
		}

		if _, err := cluster.Start(ctx); err != nil {
			return fmt.Errorf("unable to start sqldb cluster: %w", err)
		}
		rs.mutex.Lock()
		rs.servers[est.SQLDBResource] = cluster
		rs.mutex.Unlock()

		// Set up the database asynchronously since it can take a while.
		a.Go("Running database migrations", true, 250*time.Millisecond, func(ctx context.Context) error {
			err := cluster.SetupAndMigrate(ctx, rs.app.Root(), parse.Meta)
			if err != nil {
				rs.log.Error().Err(err).Msg("failed to setup db")
				return err
			}
			return nil
		})

		return nil
	}
}

// GetSQLCluster returns the SQL cluster
func (rs *ResourceServices) GetSQLCluster() *sqldb.Cluster {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if cluster, found := rs.servers[est.SQLDBResource]; found {
		return cluster.(*sqldb.Cluster)
	}
	return nil
}

func isDockerRunning(ctx context.Context) bool {
	err := exec.CommandContext(ctx, "docker", "info").Run()
	return err == nil
}
