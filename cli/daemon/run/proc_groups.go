package run

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/internal/sym"
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

	Gateway *Proc // the API gateway process

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

// parseSymTable parses the symbol table of the binary at binPath
// and stores the result in p.sym and p.symErr.
func (pg *ProcGroup) parseSymTable(binPath string) {
	parse := func() (*sym.Table, error) {
		f, err := os.Open(binPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
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
func (pg *ProcGroup) newProc(processName string) (*Proc, error) {
	port, err := allocatePort()
	if err != nil {
		return nil, errors.Wrap(err, "failed to allocate port for all-in-one proc")
	}

	dst := &url.URL{
		Scheme: "http",
		Host:   "127.0.0.1:" + strconv.Itoa(port),
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
		group:         pg,
		log:           pg.log.With().Str("proc", processName).Logger(),
		containerPort: port,
		httpProxy:     proxy,
		exit:          make(chan struct{}),
	}

	pg.procMu.Lock()
	pg.allProcesses = append(pg.allProcesses, p)
	pg.procMu.Unlock()

	return p, nil
}

func (pg *ProcGroup) NewAllInOneProc(binPath string, environ []string, secrets map[string]string, configs map[string]string) error {
	p, err := pg.newProc("all-in-one")
	if err != nil {
		return err
	}
	pg.Gateway = p

	runtimeCfg, err := pg.Run.Mgr.generateConfig(generateConfigParams{
		App:           pg.Run.App,
		RM:            pg.Run.ResourceManager,
		Meta:          pg.Meta,
		ForTests:      false,
		AuthKey:       pg.authKey,
		APIBaseURL:    "http://" + pg.Run.ListenAddr,
		ConfigAppID:   pg.Run.ID,
		ConfigEnvID:   pg.ID,
		ExternalCalls: experiments.ExternalCalls.Enabled(pg.Experiments),
	})
	if err != nil {
		return errors.Wrap(err, "failed to generate runtime config")
	}
	runtimeJSON, _ := json.Marshal(runtimeCfg)

	cmd := exec.CommandContext(pg.ctx, binPath)
	envs := append([]string{
		"ENCORE_RUNTIME_CONFIG=" + base64.RawURLEncoding.EncodeToString(runtimeJSON),
		"ENCORE_APP_SECRETS=" + encodeSecretsEnv(secrets),
		"PORT=" + strconv.Itoa(p.containerPort),
	},
		environ...,
	)
	for serviceName, cfgString := range configs {
		envs = append(envs, "ENCORE_CFG_"+strings.ToUpper(serviceName)+"="+base64.RawURLEncoding.EncodeToString([]byte(cfgString)))
	}

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

// Proc represents a single Encore process running within a [ProcGroup].
type Proc struct {
	group *ProcGroup     // The group this process belongs to
	log   zerolog.Logger // The logger for this process
	exit  chan struct{}  // closed when the process has exited
	cmd   *exec.Cmd      // The command for this specific process

	containerPort int                    // The port the HTTP server of the process should listen on
	httpProxy     *httputil.ReverseProxy // The reverse proxy for the HTTP server of the process

	// The following fields are only valid after Start() has been called.
	Started   atomic.Bool // whether the process has started
	StartedAt time.Time   // when the process started
	Pid       int         // the OS process id
}

// Start starts the process and returns immediately.
//
// If the process has already been started, this is a no-op.
func (p *Proc) Start() error {
	if !p.Started.CompareAndSwap(false, true) {
		return nil
	}

	p.group.procMu.Lock()
	defer p.group.procMu.Unlock()

	if err := p.cmd.Start(); err != nil {
		return errors.Wrap(err, "could not start process")
	}
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

	timer := time.NewTimer(10 * time.Second)
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
	_ = p.cmd.Process.Kill()
}

// allocatePort allocates a random, unused TCP port.
func allocatePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}
