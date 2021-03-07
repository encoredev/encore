package run

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"

	"encr.dev/cli/daemon/secret"
)

// BasePort is the default port Encore apps start listening on.
const BasePort = 4060

// Manager manages the set of running applications.
type Manager struct {
	RuntimePort int // port for Encore runtime
	DBProxyPort int // port for sqldb proxy
	DashPort    int // port for dev dashboard
	Secret      *secret.Manager

	listeners []EventListener

	mu   sync.Mutex
	runs map[string]*Run // id -> run
}

// EventListener is the interface for listening to events
// about running apps.
type EventListener interface {
	// OnStart is called when a run starts.
	OnStart(r *Run)
	// OnStop is called when a run stops.
	OnStop(r *Run)
	// OnStdout is called when a run outputs something on stdout.
	OnStdout(r *Run, out []byte)
	// OnStderr is called when a run outputs something on stderr.
	OnStderr(r *Run, out []byte)
}

// FindProc finds the proc with the given id.
// It reports nil if no such proc was found.
func (mgr *Manager) FindProc(procID string) *Proc {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	for _, run := range mgr.runs {
		if p := run.Proc(); p.ID == procID {
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
		if run.AppID == appID {
			return run
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

	sort.Slice(runs, func(i, j int) bool { return runs[i].AppID < runs[j].AppID })
	return runs
}

// newListener attempts to find an unused port at BasePort or above
// and opens a TCP listener on localhost for that port.
func (mgr *Manager) newListener() (ln net.Listener, port int, err error) {
	// Try up to 10 ports, plus however many processes we have
	mgr.mu.Lock()
	n := len(mgr.runs)
	mgr.mu.Unlock()
	for i := 0; i < (10 + n); i++ {
		port := BasePort + i
		ln, err := net.Listen("tcp", "localhost:"+strconv.Itoa(port))
		if err == nil {
			return ln, port, nil
		}
	}
	return nil, 0, fmt.Errorf("could not find available port in %d-%d", BasePort, BasePort+(n+9))
}

// AddListener adds an event listener to mgr.
// It must be called before starting the first run.
func (mgr *Manager) AddListener(ln EventListener) {
	mgr.listeners = append(mgr.listeners, ln)
}

func (mgr *Manager) runStdout(r *Run, out []byte) {
	// Make sure the run has started before we start outputting
	<-r.started
	for _, ln := range mgr.listeners {
		ln.OnStdout(r, out)
	}
}

func (mgr *Manager) runStderr(r *Run, out []byte) {
	// Make sure the run has started before we start outputting
	<-r.started
	for _, ln := range mgr.listeners {
		ln.OnStderr(r, out)
	}
}
