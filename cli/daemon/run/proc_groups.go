package run

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/hashicorp/yamux"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/internal/sym"
	"encr.dev/cli/internal/xos"
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

func (pg *ProcGroup) NewAllInOneProc(binPath string, environ []string, secrets map[string]string, configs map[string]string) error {
	p := &Proc{
		group: pg,
		log:   pg.log.With().Str("proc", "all-in-one").Logger(),
		exit:  make(chan struct{}),
	}
	pg.Gateway = p

	pg.procMu.Lock()
	pg.allProcesses = append(pg.allProcesses, p)
	pg.procMu.Unlock()

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

	// Set up extra file descriptors for communicating requests/responses:
	// - reqRd is for reading incoming requests (handed over procchild)
	// - reqWr is for writing incoming requests
	// - respRd is for reading responses
	// - respWr is for writing responses (handed over to proc)
	reqRd, reqWr, err1 := os.Pipe()
	respRd, respWr, err2 := os.Pipe()
	defer func() {
		// Close all the files if we return an error.
		if err != nil {
			closeAll(reqRd, reqWr, respRd, respWr)
		}
	}()
	if err := firstErr(err1, err2); err != nil {
		return errors.Wrap(err, "could not create request & response pipes")
	} else if err := xos.ArrangeExtraFiles(cmd, reqRd, respWr); err != nil {
		return errors.Wrap(err, "could not handle over request & response pipes")
	}
	p.reqWr = reqWr
	p.respRd = respRd
	p.closeOnStart = []io.Closer{reqRd, respWr}

	rwc := &struct {
		io.ReadCloser
		io.Writer
	}{
		ReadCloser: io.NopCloser(respRd),
		Writer:     reqWr,
	}
	p.Client, err = yamux.Client(rwc, yamux.DefaultConfig())
	if err != nil {
		return errors.Wrap(err, "could not initialize connection")
	}

	return nil
}

// Proc represents a single Encore process running within a [ProcGroup].
type Proc struct {
	group *ProcGroup     // The group this process belongs to
	log   zerolog.Logger // The logger for this process
	exit  chan struct{}  // closed when the process has exited
	cmd   *exec.Cmd      // The command for this specific process

	Started   atomic.Bool    // whether the process has started
	StartedAt time.Time      // when the process started
	Pid       int            // the OS process id
	Client    *yamux.Session // the client connection to the process

	reqWr        *os.File
	respRd       *os.File
	closeOnStart []io.Closer
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

	// Close the files we handed over to the child.
	closeAll(p.closeOnStart...)

	// Start watching the process for when it quits.
	go func() {
		defer close(p.exit)
		defer closeAll(p.reqWr, p.respRd)

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
	_ = p.reqWr.Close()

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

// Kill causes the Process to exit immediately. Kill does not wait until
// the Process has actually exited. This only kills the Process itself,
// not any other processes it may have started.
func (p *Proc) Kill() {
	_ = p.cmd.Process.Kill()
}
