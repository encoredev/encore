package run

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"
	"golang.org/x/net/http2"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/internal/sym"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/noopgateway"
	"encr.dev/pkg/noopgwdesc"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

type procGroupOptions struct {
	Ctx         context.Context
	ProcID      string           // unique process id
	Run         *Run             // the run the process belongs to
	Meta        *meta.Data       // app metadata snapshot
	Experiments *experiments.Set // enabled experiments
	AuthKey     config.EncoreAuthKey
	Logger      RunLogger
	WorkingDir  string
	ConfigGen   *RuntimeConfigGenerator
}

func newProcGroup(opts procGroupOptions) *ProcGroup {
	p := &ProcGroup{
		ID:          opts.ProcID,
		Run:         opts.Run,
		Meta:        opts.Meta,
		Experiments: opts.Experiments,
		workingDir:  opts.WorkingDir,
		ctx:         opts.Ctx,
		logger:      opts.Logger,
		log:         opts.Run.log.With().Str("proc_id", opts.ProcID).Logger(),
		ConfigGen:   opts.ConfigGen,

		symParsed: make(chan struct{}),
		Services:  make(map[string]*Proc),
		Gateways:  make(map[string]*Proc),
		authKey:   opts.AuthKey,
	}

	p.procCond.L = &p.procMu
	return p
}

// ProcGroup represents a running Encore application
//
// It is a collection of [Proc]'s that are all part of the same application,
// where each [Proc] represents a one or more services or an API gateway.
type ProcGroup struct {
	ID          string           // unique process id
	Run         *Run             // the run the process belongs to
	Meta        *meta.Data       // app metadata snapshot
	Experiments *experiments.Set // enabled experiments

	Gateways map[string]*Proc // the gateway processes, by name (if any)
	Services map[string]*Proc // all the service processes by name

	ConfigGen *RuntimeConfigGenerator // generates runtime configuration

	procMu       sync.Mutex // protects both allProcesses and runningProcs
	procCond     sync.Cond  // used to signal a change in runningProcs
	allProcesses []*Proc    // all processes in the group
	runningProcs uint32     // number of running processes

	ctx        context.Context
	logger     RunLogger
	log        zerolog.Logger
	workingDir string

	// Used for proxying requests when there is no gateway.
	noopGW *noopgateway.Gateway

	authKey   config.EncoreAuthKey
	sym       *sym.Table
	symErr    error
	symParsed chan struct{} // closed when sym and symErr are set
}

func (pg *ProcGroup) ProxyReq(w http.ResponseWriter, req *http.Request) {
	// Currently we only support proxying to the default gateway.
	// Need to rethink how this should work when we support multiple gateways.
	if gw, ok := pg.Gateways["api-gateway"]; ok {
		gw.ProxyReq(w, req)
	} else {
		pg.noopGW.ServeHTTP(w, req)
	}
}

// Done returns a channel that is closed when all processes in the group have exited.
func (pg *ProcGroup) Done() <-chan struct{} {
	c := make(chan struct{})
	go func() {
		pg.procMu.Lock()
		defer pg.procMu.Unlock()

		for pg.runningProcs > 0 {
			// If we have more than one process, wait for one to exit
			pg.procCond.Wait()
		}

		close(c)
	}()

	return c
}

// Start starts all the processes in the group.
func (pg *ProcGroup) Start() (err error) {
	pg.procMu.Lock()
	defer pg.procMu.Unlock()

	for _, p := range pg.allProcesses {
		if err = p.start(); err != nil {
			p.Kill()
			return err
		}
	}

	pg.noopGW = newNoopGateway(pg)
	return nil
}

// Close closes the process and waits for it to shutdown.
// It can safely be called multiple times.
func (pg *ProcGroup) Close() {
	var wg sync.WaitGroup
	pg.procMu.Lock()
	wg.Add(len(pg.allProcesses))
	for _, p := range pg.allProcesses {
		go func(p *Proc) {
			p.Close()
			wg.Done()
		}(p)
	}

	pg.procMu.Unlock()

	wg.Wait()
}

// Kill kills all the processes in the group.
// It does not wait for them to exit.
func (pg *ProcGroup) Kill() {
	pg.procMu.Lock()
	defer pg.procMu.Unlock()

	for _, p := range pg.allProcesses {
		p.Kill()
	}
}

// parseSymTable parses the symbol table of the binary at binPath
// and stores the result in p.sym and p.symErr.
func (pg *ProcGroup) parseSymTable(binPath string) {
	parse := func() (*sym.Table, error) {
		f, err := os.Open(binPath)
		if err != nil {
			return nil, err
		}
		defer fns.CloseIgnore(f)
		return sym.Load(f)
	}

	defer close(pg.symParsed)
	pg.sym, pg.symErr = parse()
}

// SymTable waits for the proc's symbol table to be parsed and then returns it.
// ctx is used to cancel the wait.
func (pg *ProcGroup) SymTable(ctx context.Context) (*sym.Table, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-pg.symParsed:
		return pg.sym, pg.symErr
	}
}

// newProc creates a new process in the group and sets up the required stuff in the struct
func (pg *ProcGroup) newProc(processName string, listenAddr netip.AddrPort) (*Proc, error) {
	dst := &url.URL{
		Scheme: "http",
		Host:   listenAddr.String(),
	}
	proxy := &httputil.ReverseProxy{
		// Enable h2c for the proxy.
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(dst)

			// Copy the host head over.
			r.Out.Host = r.In.Host

			// Add the auth key unless the test header is set.
			if r.Out.Header.Get(TestHeaderDisablePlatformAuth) == "" {
				addAuthKeyToRequest(r.Out, pg.authKey)
			}
		},
	}

	p := &Proc{
		group:      pg,
		log:        pg.log.With().Str("proc", processName).Logger(),
		listenAddr: listenAddr,
		httpProxy:  proxy,
		exit:       make(chan struct{}),
	}

	pg.procMu.Lock()
	pg.allProcesses = append(pg.allProcesses, p)
	pg.procMu.Unlock()

	return p, nil
}

func (pg *ProcGroup) NewAllInOneProc(spec builder.Cmd, listenAddr netip.AddrPort, env []string) error {
	p, err := pg.newProc("all-in-one", listenAddr)
	if err != nil {
		return err
	}

	// Append both the command-specific env and the base environment.
	env = append(env, spec.Env...)

	// This is safe since the command comes from our build.
	// nosemgrep go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(pg.ctx, spec.Command[0], spec.Command[1:]...)
	cmd.Env = env
	cmd.Dir = filepath.Join(pg.Run.App.Root(), pg.workingDir)

	// Proxy stdout and stderr to the given app logger, if any.
	if l := pg.logger; l != nil {
		cmd.Stdout = newLogWriter(pg.Run, l.RunStdout)
		cmd.Stderr = newLogWriter(pg.Run, l.RunStderr)
	}

	p.cmd = cmd

	// Assign all the gateways to this process.
	for _, gw := range pg.Meta.Gateways {
		pg.Gateways[gw.EncoreName] = p
	}

	return nil
}

func (pg *ProcGroup) NewProcForService(serviceName string, listenAddr netip.AddrPort, spec builder.Cmd, env []string) error {
	if !listenAddr.IsValid() {
		return errors.New("invalid listen address")
	}

	p, err := pg.newProc(serviceName, listenAddr)
	if err != nil {
		return err
	}
	pg.Services[serviceName] = p

	// Append both the command-specific env and the base environment.
	env = append(env, spec.Env...)

	// This is safe since the command comes from our build.
	// nosemgrep go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(pg.ctx, spec.Command[0], spec.Command[1:]...)
	cmd.Env = env
	cmd.Dir = filepath.Join(pg.Run.App.Root(), pg.workingDir)

	// Proxy stdout and stderr to the given app logger, if any.
	if l := pg.logger; l != nil {
		cmd.Stdout = newLogWriter(pg.Run, l.RunStdout)
		cmd.Stderr = newLogWriter(pg.Run, l.RunStderr)
	}

	p.cmd = cmd

	return nil
}

func (pg *ProcGroup) NewProcForGateway(gatewayName string, listenAddr netip.AddrPort, spec builder.Cmd, env []string) error {
	if !listenAddr.IsValid() {
		return errors.New("invalid listen address")
	}

	p, err := pg.newProc("gateway-"+gatewayName, listenAddr)
	if err != nil {
		return err
	}
	pg.Gateways[gatewayName] = p

	// Append both the command-specific env and the base environment.
	env = append(env, spec.Env...)

	// This is safe since the command comes from our build.
	// nosemgrep go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(pg.ctx, spec.Command[0], spec.Command[1:]...)
	cmd.Env = env
	cmd.Dir = filepath.Join(pg.Run.App.Root(), pg.workingDir)

	// Proxy stdout and stderr to the given app logger, if any.
	if l := pg.logger; l != nil {
		cmd.Stdout = newLogWriter(pg.Run, l.RunStdout)
		cmd.Stderr = newLogWriter(pg.Run, l.RunStderr)
	}

	p.cmd = cmd

	return nil
}

type warning struct {
	Title string
	Help  string
}

func (pg *ProcGroup) Warnings() (rtn []warning) {
	if missing := pg.ConfigGen.MissingSecrets(); len(missing) > 0 {
		rtn = append(rtn, warning{
			Title: "secrets not defined: " + strings.Join(missing, ", "),
			Help:  "undefined secrets are left empty for local development only.\nsee https://encore.dev/docs/primitives/secrets for more information",
		})
	}

	return rtn
}

// Proc represents a single Encore process running within a [ProcGroup].
type Proc struct {
	group *ProcGroup     // The group this process belongs to
	log   zerolog.Logger // The logger for this process
	exit  chan struct{}  // closed when the process has exited
	cmd   *exec.Cmd      // The command for this specific process

	listenAddr netip.AddrPort         // The port the HTTP server of the process should listen on
	httpProxy  *httputil.ReverseProxy // The reverse proxy for the HTTP server of the process

	// The following fields are only valid after Start() has been called.
	Started   atomic.Bool // whether the process has started
	StartedAt time.Time   // when the process started
	Pid       int         // the OS process id
}

// Start starts the process and returns immediately.
//
// If the process has already been started, this is a no-op.
func (p *Proc) Start() error {
	p.group.procMu.Lock()
	defer p.group.procMu.Unlock()

	return p.start()
}

// start starts the process and returns immediately
//
// It must be called while locked under the p.group.procMu lock.
func (p *Proc) start() error {
	if !p.Started.CompareAndSwap(false, true) {
		return nil
	}

	if err := p.cmd.Start(); err != nil {
		return errors.Wrap(err, "could not start process")
	}
	p.log.Info().Str("addr", p.listenAddr.String()).Msg("process started")
	p.group.runningProcs++

	p.Pid = p.cmd.Process.Pid
	p.StartedAt = time.Now()

	// Start watching the process for when it quits.
	go func() {
		defer close(p.exit)

		// Wait for the process to exit.
		err := p.cmd.Wait()
		if err != nil && p.group.ctx.Err() == nil {
			p.log.Error().Err(err).Msg("process exited with error")
		} else {
			p.log.Info().Msg("process exited successfully")
		}

		// Flush the logs in case the output did not end in a newline.
		for _, w := range [...]io.Writer{p.cmd.Stdout, p.cmd.Stderr} {
			if w != nil {
				w.(*logWriter).Flush()
			}
		}
	}()

	// When the process exits, decrement the running count for the group
	// and wake up any goroutines waiting for on the running count to shrink
	go func() {
		<-p.exit
		p.group.procMu.Lock()
		defer p.group.procMu.Unlock()
		p.group.runningProcs--

		p.group.procCond.Broadcast()
	}()

	return nil
}

// Close closes the process and waits for it to exit.
// It is safe to call Close multiple times.
func (p *Proc) Close() {
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		// If there's an error sending the signal, just kill the process.
		// This might happen because Interrupt is not supported on Windows.
		p.Kill()
	}

	timer := time.NewTimer(gracefulShutdownTime + (500 * time.Millisecond))
	defer timer.Stop()

	select {
	case <-p.exit:
		// already exited
	case <-timer.C:
		p.group.log.Error().Msg("timed out waiting for process to exit; killing")
		p.Kill()
		<-p.exit
	}
}

// ProxyReq proxies the request to the Encore app.
func (p *Proc) ProxyReq(w http.ResponseWriter, req *http.Request) {
	p.httpProxy.ServeHTTP(w, req)
}

// Kill causes the Process to exit immediately. Kill does not wait until
// the Process has actually exited. This only kills the Process itself,
// not any other processes it may have started.
func (p *Proc) Kill() {
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
}

// pollUntilProcessIsListening polls the listen address until
// the process is actively listening, five seconds have passed,
// or the context is canceled, whichever happens first.
//
// It reports true if the process is listening on return, false otherwise.
func (p *Proc) pollUntilProcessIsListening(ctx context.Context) (ok bool) {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 50 * time.Millisecond
	b.MaxInterval = 250 * time.Millisecond
	b.MaxElapsedTime = 5 * time.Second

	err := backoff.Retry(func() error {
		if err := ctx.Err(); err != nil {
			return backoff.Permanent(err)
		}

		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", p.listenAddr.String())
		if err == nil {
			_ = conn.Close()
		}
		return err
	}, b)
	return err == nil
}

func newNoopGateway(pg *ProcGroup) *noopgateway.Gateway {
	svcDiscovery := make(map[noopgateway.ServiceName]string)
	for _, svc := range pg.Meta.Svcs {
		if proc, ok := pg.Services[svc.Name]; ok {
			svcDiscovery[noopgateway.ServiceName(svc.Name)] = proc.listenAddr.String()
		}
	}

	desc := noopgwdesc.Describe(pg.Meta, svcDiscovery)
	gw := noopgateway.New(desc)

	gw.Rewrite = func(rp *httputil.ProxyRequest) {
		// Copy the host head over.
		rp.Out.Host = rp.In.Host

		// Add the auth key unless the test header is set.
		if rp.Out.Header.Get(TestHeaderDisablePlatformAuth) == "" {
			addAuthKeyToRequest(rp.Out, pg.authKey)
		}
	}

	return gw
}
