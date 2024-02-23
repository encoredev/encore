package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"
	"go4.org/syncutil"

	"encr.dev/pkg/noopgateway"
	runtimev1 "encr.dev/proto/encore/runtime/v1"
)

// Config is the configuration used by the supervisor.
type Config struct {
	Procs []Proc `json:"procs"`

	// NoopGateways are the noop-gateways to start up,
	// keyed by gateway name.
	NoopGateways map[string]*noopgateway.Description
}

// Proc represents a supervised proc.
type Proc struct {
	// ID is a unique id representing this process.
	ID string `json:"id"`

	// Command and arguments for running the proc.
	Command []string `json:"command"`

	// Extra environment variables to pass.
	Env []string `json:"env"`

	// supervisedProc and gateway names included in this proc.
	Services []string `json:"services"`
	Gateways []string `json:"gateways"`
}

// New creates a new supervisor.
func New(cfg *Config, rtCfg *runtimev1.RuntimeConfig) (*Supervisor, error) {
	log := zerolog.New(os.Stderr).With().Timestamp().Logger()

	super := &Supervisor{
		cfg:   cfg,
		rt:    rtCfg,
		procs: make(map[string]*supervisedProc),
		log:   log,
	}

	return super, nil
}

type Supervisor struct {
	cfg   *Config
	rt    *runtimev1.RuntimeConfig
	procs map[string]*supervisedProc

	buildInfoOnce syncutil.Once
	buildInfo     BuildInfo

	log zerolog.Logger
}

// Run runs the supervisor. It returns immediately on error,
// and otherwise blocks until ctx is canceled.
func (s *Supervisor) Run(ctx context.Context) error {
	// Start up the procs to supervise.
	for i, proc := range s.cfg.Procs {
		services := strings.Join(proc.Services, ",")
		gateways := strings.Join(proc.Gateways, ",")

		logger := s.log.With().
			Str("services", services).
			Str("gateways", gateways).
			Str("proc_id", proc.ID).
			Logger()

		port := 12000 + i
		sp := &supervisedProc{
			super: s,
			proc:  proc,
			log:   logger,
			port:  port,
		}
		s.procs[proc.ID] = sp
		go sp.Supervise()
	}

	// Start up the noop-gateways.
	go s.runNoopGateway(ctx, "noop")

	<-ctx.Done()
	return nil
}

// runNoopGateway runs a noop-gateway until ctx is canceled.
func (s *Supervisor) runNoopGateway(ctx context.Context, name string) {
	logger := s.log.With().Str("gateway", name).Logger()
	logger.Info().Msg("starting noop-gateway")

	// Find the default gateway
	var targetGW *supervisedProc
	for _, proc := range s.procs {
		if len(proc.proc.Gateways) > 0 {
			targetGW = proc
			break
		}
	}
	if targetGW == nil {
		logger.Error().Msg("no gateway found")
		return
	}

	target := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("localhost:%d", targetGW.port),
	}
	rp := httputil.NewSingleHostReverseProxy(target)
	ln, err := listenGateway()
	if err != nil {
		logger.Error().Err(err).Msg("listen")
		return
	}
	logger.Info().Msgf("listening on %s", ln.Addr().String())
	srv := &http.Server{
		BaseContext: func(_ net.Listener) context.Context { return ctx },
		Handler:     rp,
	}
	if err := srv.Serve(ln); err != nil {
		logger.Error().Err(err).Msg("serve")
		return
	}
}

func listenGateway() (net.Listener, error) {
	listenAddr := os.Getenv("ENCORE_LISTEN_ADDR")
	if listenAddr != "" {
		addrPort, err := netip.ParseAddrPort(listenAddr)
		if err != nil {
			return nil, err
		}
		return net.Listen("tcp", addrPort.String())
	}

	port, _ := strconv.Atoi(os.Getenv("PORT"))
	if port == 0 {
		port = 8080
	}
	return net.Listen("tcp", ":"+strconv.Itoa(port))
}

// supervisedProc is a supervised process.
type supervisedProc struct {
	super   *Supervisor
	proc    Proc
	log     zerolog.Logger
	healthy atomic.Bool
	port    int
}

// Healthy reports whether the service is currently healthy.
func (s *supervisedProc) Healthy() bool {
	return s.healthy.Load()
}

// Supervise supervises the service, starting it and restarting it if it exits.
func (p *supervisedProc) Supervise() {
	const (
		minSleep = 100 * time.Millisecond
		maxSleep = 10 * time.Second
	)

	var (
		mu         sync.Mutex
		generation uint64 = 1
		retrySleep        = minSleep
	)

	errSleep := func(msg string, err error) {
		// Mark the service as unhealthy.
		p.healthy.Store(false)

		// Increment retry sleep and generation.
		mu.Lock()
		generation++
		toSleep := retrySleep
		retrySleep *= 2
		if retrySleep > maxSleep {
			retrySleep = maxSleep
		}
		mu.Unlock()

		p.log.Error().Err(err).Msgf("%s: %v, retrying in %v", msg, err, toSleep)
		time.Sleep(toSleep)
	}

	// If this proc is using the sidecar, add its environment variables.
	env := os.Environ()
	env = append(env, p.proc.Env...)

	// Add the port to listen on.
	env = append(env, fmt.Sprintf("PORT=%d", p.port))

	for {
		mu.Lock()
		currGen := generation
		mu.Unlock()

		cmd := exec.Command(p.proc.Command[0], p.proc.Command[1:]...)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		p.log.Info().Msg("starting proc")
		if err := cmd.Start(); err != nil {
			errSleep("service startup failed", err)
			continue
		}

		// Check in on the process after a few seconds and reset the retry sleep if it's still alive.
		go func() {
			time.Sleep(1 * time.Second)
			mu.Lock()
			defer mu.Unlock()
			if generation == currGen {
				// Still alive. Reset the retry sleep.
				retrySleep = minSleep
				p.healthy.Store(true)
			}
		}()

		if err := cmd.Wait(); err != nil {
			errSleep("proc exited", err)
			continue
		}
	}
}

func readBuildInfo() (BuildInfo, error) {
	var info BuildInfo
	data, err := os.ReadFile("/encore/build-info.json")
	if err == nil {
		err = json.Unmarshal(data, &info)
	}
	return info, errors.Wrap(err, "read build info")
}

type BuildInfo struct {
	// The version of Encore with which the app was compiled.
	// This is string is for informational use only, and its format should not be relied on.
	EncoreCompiler string

	// AppCommit describes the commit of the app.
	AppCommit CommitInfo
}

type CommitInfo struct {
	Revision    string
	Uncommitted bool
}

func (ci CommitInfo) AsRevisionString() string {
	if ci.Uncommitted {
		return fmt.Sprintf("%s-modified", ci.Revision)
	}
	return ci.Revision
}
