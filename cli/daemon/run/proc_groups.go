package run

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/internal/sym"
	"encr.dev/pkg/fns"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// ProcGroup represents a running Encore application
//
// It is a collection of [Proc]'s that are all part of the same application,
// where each [Proc] represents a one or more services or a API gateway.
type ProcGroup struct {
	ID          string           // unique process id
	Run         *Run             // the run the process belongs to
	Meta        *meta.Data       // app metadata snapshot
	Experiments *experiments.Set // enabled experiments

	Gateway  *Proc            // the API gateway process
	Services map[string]*Proc // all the service processes by name

	EnvGenerator *RuntimeEnvGenerator // generates runtime environment variables

	procMu       sync.Mutex // protects both allProcesses and runningProcs
	procCond     sync.Cond  // used to signal a change in runningProcs
	allProcesses []*Proc    // all processes in the group
	runningProcs uint32     // number of running processes

	ctx        context.Context
	logger     RunLogger
	log        zerolog.Logger
	buildDir   string
	workingDir string

	authKey   config.EncoreAuthKey
	sym       *sym.Table
	symErr    error
	symParsed chan struct{} // closed when sym and symErr are set
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
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(dst)

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

type NewProcParams struct {
	BinPath string   // The path to the binary to run
	Environ []string // The base environment to run the process with
}

func (pg *ProcGroup) NewAllInOneProc(params *NewProcParams) error {
	ports, err := GenerateListenAddresses(pg.Run.SvcProxy, nil)
	if err != nil {
		return err
	}

	p, err := pg.newProc("all-in-one", ports.Gateway.ListenAddr)
	if err != nil {
		return err
	}
	pg.Gateway = p

	// Generate the environmental variables for the process
	envs, err := pg.EnvGenerator.ForAllInOne(ports.Gateway.ListenAddr)
	if err != nil {
		return errors.Wrap(err, "failed to generate environment variables")
	}
	envs = append(envs, params.Environ...)

	// This is safe since the command comes from our build.
	// nosemgrep go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(pg.ctx, params.BinPath)
	cmd.Env = envs
	cmd.Dir = filepath.Join(pg.Run.App.Root(), pg.workingDir)

	// Proxy stdout and stderr to the given app logger, if any.
	if l := pg.logger; l != nil {
		cmd.Stdout = newLogWriter(pg.Run, l.RunStdout)
		cmd.Stderr = newLogWriter(pg.Run, l.RunStderr)
	}

	p.cmd = cmd

	return nil
}

func (pg *ProcGroup) NewProcForService(service *meta.Service, listenAddr netip.AddrPort, params *NewProcParams) error {
	if !listenAddr.IsValid() {
		return errors.New("invalid listen address")
	}

	p, err := pg.newProc(service.Name, listenAddr)
	if err != nil {
		return err
	}
	pg.Services[service.Name] = p

	// Generate the environmental variables for the process
	envs, err := pg.EnvGenerator.ForServices(listenAddr, service)
	if err != nil {
		return errors.Wrap(err, "failed to generate environment variables")
	}
	envs = append(envs, params.Environ...)

	// This is safe since the command comes from our build.
	// nosemgrep go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(pg.ctx, params.BinPath)
	cmd.Env = envs
	cmd.Dir = filepath.Join(pg.Run.App.Root(), pg.workingDir)

	// Proxy stdout and stderr to the given app logger, if any.
	if l := pg.logger; l != nil {
		cmd.Stdout = newLogWriter(pg.Run, l.RunStdout)
		cmd.Stderr = newLogWriter(pg.Run, l.RunStderr)
	}

	p.cmd = cmd

	return nil
}

func (pg *ProcGroup) NewProcForGateway(listenAddr netip.AddrPort, params *NewProcParams) error {
	if !listenAddr.IsValid() {
		return errors.New("invalid listen address")
	}

	p, err := pg.newProc("api-gateway", listenAddr)
	if err != nil {
		return err
	}
	pg.Gateway = p

	envs, err := pg.EnvGenerator.ForGateway(listenAddr, "localhost", "127.0.0.1")
	if err != nil {
		return errors.Wrap(err, "failed to generate environment variables")
	}
	envs = append(envs, params.Environ...)

	// This is safe since the command comes from our build.
	// nosemgrep go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(pg.ctx, params.BinPath)
	cmd.Env = envs
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
	if len(pg.EnvGenerator.missingSecrets) > 0 {
		var missing []string
		for k := range pg.EnvGenerator.missingSecrets {
			missing = append(missing, k)
		}
		sort.Strings(missing)

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
	p.log.Info().Msg("process started")
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

		conn, err := net.Dial("tcp", p.listenAddr.String())
		if err == nil {
			_ = conn.Close()
		}
		return err
	}, b)
	return err == nil
}
