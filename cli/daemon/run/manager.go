package run

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/xid"

	encore "encore.dev"
	"encore.dev/appruntime/exported/config"
	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/run/infra"
	"encr.dev/cli/daemon/secret"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/pkg/errlist"
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

type generateConfigParams struct {
	App  *apps.Instance
	RM   *infra.ResourceManager
	Meta *meta.Data

	ForTests   bool
	AuthKey    config.EncoreAuthKey
	APIBaseURL string

	ConfigAppID string
	ConfigEnvID string
}

func (mgr *Manager) generateConfig(p generateConfigParams) (*config.Runtime, error) {
	envType := encore.EnvDevelopment
	if p.ForTests {
		envType = encore.EnvTest
	}

	globalCORS, err := p.App.GlobalCORS()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get global CORS")
	}

	deployID := xid.New().String()
	if p.ForTests {
		deployID = "clitest_" + deployID
	} else {
		deployID = "run_" + deployID
	}

	cfg := &config.Runtime{
		AppID:         p.ConfigAppID,
		AppSlug:       p.App.PlatformID(),
		APIBaseURL:    p.APIBaseURL,
		DeployID:      deployID,
		DeployedAt:    time.Now().UTC(), // Force UTC to not cause confusion
		EnvID:         p.ConfigEnvID,
		EnvName:       "local",
		EnvCloud:      string(encore.CloudLocal),
		EnvType:       string(envType),
		TraceEndpoint: fmt.Sprintf("http://localhost:%d/trace", mgr.RuntimePort),
		AuthKeys:      []config.EncoreAuthKey{p.AuthKey},
		CORS: &config.CORS{
			Debug: globalCORS.Debug,
			AllowOriginsWithCredentials: []string{
				// Allow all origins with credentials for local development;
				// since it's only running on localhost for development this is safe.
				config.UnsafeAllOriginWithCredentials,
			},
			AllowOriginsWithoutCredentials: []string{"*"},
			ExtraAllowedHeaders:            globalCORS.AllowHeaders,
			ExtraExposedHeaders:            globalCORS.ExposeHeaders,
			AllowPrivateNetworkAccess:      true,
		},
	}

	p.RM.UpdateConfig(cfg, p.Meta, mgr.DBProxyPort)
	return cfg, nil
}
