package run

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/xid"
	"golang.org/x/mod/modfile"

	encore "encore.dev"
	"encore.dev/appruntime/config"
	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/secret"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/parser"
	"encr.dev/pkg/errlist"
	"encr.dev/pkg/vcs"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// Manager manages the set of running applications.
type Manager struct {
	RuntimePort int // port for Encore runtime
	DBProxyPort int // port for sqldb proxy
	DashPort    int // port for dev dashboard
	Secret      *secret.Manager
	ClusterMgr  *sqldb.ClusterManager

	listeners []EventListener
	mu        sync.Mutex
	runs      map[string]*Run // id -> run
}

// EventListener is the interface for listening to events
// about running apps.
type EventListener interface {
	// OnStart is called when a run starts.
	OnStart(r *Run)
	// OnReload is called when a run reloads.
	OnReload(r *Run)
	// OnStop is called when a run stops.
	OnStop(r *Run)
	// OnStdout is called when a run outputs something on stdout.
	OnStdout(r *Run, out []byte)
	// OnStderr is called when a run outputs something on stderr.
	OnStderr(r *Run, out []byte)
	// OnError is called when a run encounters an error.
	OnError(r *Run, err *errlist.List)
}

// FindProc finds the proc with the given id.
// It reports nil if no such proc was found.
func (mgr *Manager) FindProc(procID string) *Proc {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	for _, run := range mgr.runs {
		if p := run.Proc(); p != nil && p.ID == procID {
			return p
		}
	}
	return nil
}

// FindRunByAppID finds the run with the given app id.
// It reports nil if no such run was found.
func (mgr *Manager) FindRunByAppID(appID string) *Run {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	for _, run := range mgr.runs {
		if appID == run.App.PlatformID() || appID == run.App.LocalID() {
			select {
			case <-run.Done():
				// exited
			default:
				return run
			}
		}
	}
	return nil
}

// ListRuns provides a snapshot of all runs.
func (mgr *Manager) ListRuns() []*Run {
	mgr.mu.Lock()
	runs := make([]*Run, 0, len(mgr.runs))
	for _, r := range mgr.runs {
		runs = append(runs, r)
	}
	mgr.mu.Unlock()

	sort.Slice(runs, func(i, j int) bool { return runs[i].App.PlatformOrLocalID() < runs[j].App.PlatformOrLocalID() })
	return runs
}

// AddListener adds an event listener to mgr.
// It must be called before starting the first run.
func (mgr *Manager) AddListener(ln EventListener) {
	mgr.listeners = append(mgr.listeners, ln)
}

func (mgr *Manager) RunStdout(r *Run, out []byte) {
	// Make sure the run has started before we start outputting
	<-r.started
	for _, ln := range mgr.listeners {
		ln.OnStdout(r, out)
	}
}

func (mgr *Manager) RunStderr(r *Run, out []byte) {
	// Make sure the run has started before we start outputting
	<-r.started
	for _, ln := range mgr.listeners {
		ln.OnStderr(r, out)
	}
}

func (mgr *Manager) RunError(r *Run, err *errlist.List) {
	for _, ln := range mgr.listeners {
		ln.OnError(r, err)
	}
}

type parseAppParams struct {
	App           *apps.Instance
	Environ       []string
	WorkingDir    string
	ParseTests    bool
	ScriptMainPkg string
}

// parseApp parses the app and returns the parse result.
func (mgr *Manager) parseApp(p parseAppParams) (*parser.Result, error) {
	modPath := filepath.Join(p.App.Root(), "go.mod")
	modData, err := os.ReadFile(modPath)
	if err != nil {
		return nil, err
	}
	mod, err := modfile.Parse(modPath, modData, nil)
	if err != nil {
		return nil, err
	}

	vcsRevision := vcs.GetRevision(p.App.Root())

	experiments, err := p.App.Experiments(p.Environ)
	if err != nil {
		return nil, err
	}

	cfg := &parser.Config{
		AppRoot:                  p.App.Root(),
		Experiments:              experiments,
		AppRevision:              vcsRevision.Revision,
		AppHasUncommittedChanges: vcsRevision.Uncommitted,
		ModulePath:               mod.Module.Mod.Path,
		WorkingDir:               p.WorkingDir,
		ParseTests:               p.ParseTests,
		ScriptMainPkg:            p.ScriptMainPkg,
	}

	return parser.Parse(cfg)
}

type generateConfigParams struct {
	App  *apps.Instance
	RS   *ResourceServices
	Meta *meta.Data

	ForTests   bool
	AuthKey    config.EncoreAuthKey
	APIBaseURL string

	ConfigAppID string
	ConfigEnvID string
}

func (mgr *Manager) generateConfig(p generateConfigParams) (*config.Runtime, error) {
	var (
		sqlServers []*config.SQLServer
		sqlDBs     []*config.SQLDatabase
	)
	if cluster := p.RS.GetSQLCluster(); cluster != nil {
		srv := &config.SQLServer{
			Host: "localhost:" + strconv.Itoa(mgr.DBProxyPort),
		}
		sqlServers = append(sqlServers, srv)

		for _, svc := range p.Meta.Svcs {
			if len(svc.Migrations) > 0 {
				sqlDBs = append(sqlDBs, &config.SQLDatabase{
					EncoreName:   svc.Name,
					DatabaseName: svc.Name,
					User:         "encore",
					Password:     cluster.Password,
				})
			}
		}

		// Configure max connections based on 96 connections
		// divided evenly among the databases
		maxConns := 96 / len(sqlDBs)
		for _, db := range sqlDBs {
			db.MaxConnections = maxConns
		}
	}

	var (
		pubsubProviders []*config.PubsubProvider
		pubsubTopics    map[string]*config.PubsubTopic
	)
	if nsq := p.RS.GetPubSub(); nsq != nil {
		provider := &config.PubsubProvider{
			NSQ: &config.NSQProvider{
				Host: nsq.Addr(),
			},
		}
		pubsubProviders = append(pubsubProviders, provider)
		pubsubTopics = make(map[string]*config.PubsubTopic)
		for _, t := range p.Meta.PubsubTopics {
			topicCfg := &config.PubsubTopic{
				EncoreName:    t.Name,
				ProviderID:    0,
				ProviderName:  t.Name,
				Subscriptions: make(map[string]*config.PubsubSubscription),
			}

			if t.OrderingKey != "" {
				topicCfg.OrderingKey = t.OrderingKey
			}

			for _, s := range t.Subscriptions {
				topicCfg.Subscriptions[s.Name] = &config.PubsubSubscription{
					ID:           s.Name,
					EncoreName:   s.Name,
					ProviderName: s.Name,
				}
			}

			pubsubTopics[t.Name] = topicCfg
		}
	}

	var (
		redisServers []*config.RedisServer
		redisDBs     []*config.RedisDatabase
	)
	if redis := p.RS.GetRedis(); redis != nil {
		srv := &config.RedisServer{
			Host: redis.Addr(),
		}
		redisServers = append(redisServers, srv)

		for _, cluster := range p.Meta.CacheClusters {
			redisDBs = append(redisDBs, &config.RedisDatabase{
				ServerID:   0,
				Database:   0,
				EncoreName: cluster.Name,
				KeyPrefix:  cluster.Name + "/",
			})
		}
	}

	envType := encore.EnvDevelopment
	if p.ForTests {
		envType = encore.EnvTest
	}

	globalCORS, err := p.App.GlobalCORS()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get global CORS")
	}

	metricsConfig := &config.Metrics{
		JSONBased: &config.JSONBasedMetricsProvider{},
	}

	return &config.Runtime{
		AppID:           p.ConfigAppID,
		AppSlug:         p.App.PlatformID(),
		APIBaseURL:      p.APIBaseURL,
		DeployID:        fmt.Sprintf("run_%s", xid.New()),
		DeployedAt:      time.Now().UTC(), // Force UTC to not cause confusion
		EnvID:           p.ConfigEnvID,
		EnvName:         "local",
		EnvCloud:        string(encore.CloudLocal),
		EnvType:         string(envType),
		TraceEndpoint:   fmt.Sprintf("http://localhost:%d/trace", mgr.RuntimePort),
		SQLDatabases:    sqlDBs,
		SQLServers:      sqlServers,
		PubsubProviders: pubsubProviders,
		PubsubTopics:    pubsubTopics,
		RedisServers:    redisServers,
		RedisDatabases:  redisDBs,
		AuthKeys:        []config.EncoreAuthKey{p.AuthKey},
		CORS: &config.CORS{
			Debug: globalCORS.Debug,
			AllowOriginsWithCredentials: []string{
				// Allow all origins with credentials for local development;
				// since it's only running on localhost for development this is safe.
				config.UnsafeAllOriginWithCredentials,
			},
			AllowOriginsWithoutCredentials: []string{"*"},
			ExtraAllowedHeaders:            globalCORS.AllowHeaders,
			AllowPrivateNetworkAccess:      true,
		},
		Metrics: metricsConfig,
	}, nil
}
