// Package run starts and tracks running Encore applications.
package run

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	mathrand "math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/hashicorp/yamux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encore.dev/appruntime/exported/config"
	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/internal/sym"
	"encr.dev/cli/daemon/run/infra"
	"encr.dev/cli/daemon/secret"
	"encr.dev/cli/internal/xos"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/experiments"
	"encr.dev/pkg/vcs"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// Run represents a running Encore application.
type Run struct {
	ID              string // unique ID for this instance of the running app
	App             *apps.Instance
	ListenAddr      string // the address the app is listening on
	ResourceManager *infra.ResourceManager

	builder builder.Impl
	log     zerolog.Logger
	Mgr     *Manager
	params  *StartParams
	secrets *secret.LoadResult

	ctx     context.Context // ctx is closed when the run is to exit
	proc    atomic.Value    // current process
	exited  chan struct{}   // exit is closed when the run has fully exited
	started chan struct{}   // started is closed once the run has fully started
}

// StartParams groups the parameters for the Run method.
type StartParams struct {
	// App is the app to start.
	App *apps.Instance

	// WorkingDir is the working dir, for formatting
	// error messages with relative paths.
	WorkingDir string

	// Watch enables watching for code changes for live reloading.
	Watch bool

	Listener   net.Listener // listener to use
	ListenAddr string       // address we're listening on

	// Environ are the environment variables to set for the running app,
	// in the same format as os.Environ().
	Environ []string

	// The Ops tracker being used for this run
	OpsTracker *optracker.OpTracker

	// Debug specifies to compile the application for debugging.
	Debug bool
}

// Start starts the application.
// Its lifetime is bounded by ctx.
func (mgr *Manager) Start(ctx context.Context, params StartParams) (run *Run, err error) {
	run = &Run{
		ID:              GenID(),
		App:             params.App,
		ResourceManager: infra.NewResourceManager(params.App, mgr.ClusterMgr, params.Environ, false),
		ListenAddr:      params.ListenAddr,

		log:     log.With().Str("app_id", params.App.PlatformOrLocalID()).Logger(),
		Mgr:     mgr,
		params:  &params,
		secrets: mgr.Secret.Load(params.App),
		ctx:     ctx,
		exited:  make(chan struct{}),
		started: make(chan struct{}),
	}
	defer func(r *Run) {
		// Stop all the resource servers if we exit due to an error
		if err != nil {
			r.ResourceManager.StopAll()
		}
	}(run)

	// Add the run to our map before starting to avoid
	// racing with initialization (though it's unlikely to ever matter).
	mgr.mu.Lock()
	if mgr.runs == nil {
		mgr.runs = make(map[string]*Run)
	}
	mgr.runs[run.ID] = run
	mgr.mu.Unlock()

	if err := run.start(params.Listener, params.OpsTracker); err != nil {
		if errList := asErrorList(err); errList != nil {
			return nil, errList
		}
		return nil, err
	}

	if params.Watch {
		if err := mgr.watch(run); err != nil {
			return nil, err
		}
	}

	return run, nil
}

// RunLogger is the interface for listening to run logs.
// The log methods are called for each logline on stdout and stderr respectively.
type RunLogger interface {
	RunStdout(r *Run, line []byte)
	RunStderr(r *Run, line []byte)
}

// Proc returns the current running process.
// It may have already exited.
// If the proc has not yet started it may return nil.
func (r *Run) Proc() *Proc {
	p, _ := r.proc.Load().(*Proc)
	return p
}

func (r *Run) StoreProc(p *Proc) {
	r.proc.Store(p)
}

// Done returns a channel that is closed when the run is closed.
func (r *Run) Done() <-chan struct{} {
	return r.exited
}

// Reload rebuilds the app and, if successful,
// starts a new proc and switches over.
func (r *Run) Reload() error {
	err := r.buildAndStart(r.ctx, nil)
	if err != nil {
		return err
	}

	for _, ln := range r.Mgr.listeners {
		ln.OnReload(r)
	}

	return nil
}

// start starts the application and serves requests over HTTP using ln.
func (r *Run) start(ln net.Listener, tracker *optracker.OpTracker) (err error) {
	defer func() {
		if err != nil {
			// This is closed below when err == nil,
			// so handle the other cases.
			close(r.started)
			close(r.exited)
		}
	}()

	err = r.buildAndStart(r.ctx, tracker)
	if err != nil {
		return err
	}

	// Below this line the function must never return an error
	// in order to only ensure we Close r.exited exactly once.

	go func() {
		for _, ln := range r.Mgr.listeners {
			ln.OnStart(r)
		}
		close(r.started)
	}()

	// Run the http server until the app exits.
	srv := &http.Server{Addr: ln.Addr().String(), Handler: r}
	go func() {
		if err := srv.Serve(ln); err != http.ErrServerClosed {
			r.log.Error().Err(err).Msg("could not serve")
		}
	}()
	go func() {
		<-r.ctx.Done()
		srv.Close()
	}()

	// Monitor the running proc and Close the app when it exits.
	go func() {
		for {
			p := r.proc.Load().(*Proc)
			<-p.Done()
			// p exited, but it could have been a reload.
			// Check to make sure p is still the active proc.
			p2 := r.proc.Load().(*Proc)
			if p2 == p {
				// We're done.
				for _, ln := range r.Mgr.listeners {
					ln.OnStop(r)
				}
				close(r.exited)
				return
			}
		}
	}()
	return nil
}

// buildAndStart builds the app, starts the proc, and cleans up
// the build dir when it exits.
// The proc exits when ctx is canceled.
func (r *Run) buildAndStart(ctx context.Context, tracker *optracker.OpTracker) error {
	// Return early if the ctx is already canceled.
	if err := ctx.Err(); err != nil {
		return err
	}

	jobs := optracker.NewAsyncBuildJobs(ctx, r.App.PlatformOrLocalID(), tracker)

	// Parse the app source code
	// Parse the app to figure out what infrastructure is needed.
	start := time.Now()
	parseOp := tracker.Add("Building Encore application graph", start)
	topoOp := tracker.Add("Analyzing service topology", start)

	expSet, err := r.App.Experiments(r.params.Environ)
	if err != nil {
		return err
	}

	if r.builder == nil {
		r.builder = builderimpl.Resolve(expSet)
	}

	vcsRevision := vcs.GetRevision(r.App.Root())
	buildInfo := builder.BuildInfo{
		BuildTags:          builder.LocalBuildTags,
		CgoEnabled:         true,
		StaticLink:         false,
		Debug:              r.params.Debug,
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		KeepOutput:         false,
		Revision:           vcsRevision.Revision,
		UncommittedChanges: vcsRevision.Uncommitted,
	}

	parse, err := r.builder.Parse(ctx, builder.ParseParams{
		Build:       buildInfo,
		App:         r.App,
		Experiments: expSet,
		WorkingDir:  r.params.WorkingDir,
		ParseTests:  false,
	})
	if err != nil {
		tracker.Fail(parseOp, err)
		return err
	}
	tracker.Done(parseOp, 500*time.Millisecond)
	tracker.Done(topoOp, 300*time.Millisecond)

	r.ResourceManager.StartRequiredServices(jobs, parse.Meta)

	var build *builder.CompileResult
	jobs.Go("Compiling application source code", false, 0, func(ctx context.Context) (err error) {
		build, err = r.builder.Compile(ctx, builder.CompileParams{
			Build:       buildInfo,
			App:         r.App,
			Parse:       parse,
			OpTracker:   tracker,
			Experiments: expSet,
			WorkingDir:  r.params.WorkingDir,
			CueMeta: &cueutil.Meta{
				APIBaseURL: fmt.Sprintf("http://%s", r.ListenAddr),
				EnvName:    "local",
				EnvType:    cueutil.EnvType_Development,
				CloudType:  cueutil.CloudType_Local,
			},
		})
		if err != nil {
			return errors.Wrap(err, "compile error")
		}
		return nil
	})
	defer func() {
		if err != nil && build != nil {
			os.RemoveAll(build.Dir)
		}
	}()

	var secrets map[string]string
	if usesSecrets(parse.Meta) {
		jobs.Go("Fetching application secrets", true, 150*time.Millisecond, func(ctx context.Context) error {
			data, err := r.secrets.Get(ctx, expSet)
			if err != nil {
				return err
			}
			secrets = data.Values
			return nil
		})
	}

	if err := jobs.Wait(); err != nil {
		return err
	}

	startOp := tracker.Add("Starting Encore application", start)
	newProcess, err := r.StartProc(&StartProcParams{
		Ctx:            ctx,
		BuildDir:       build.Dir,
		BinPath:        build.Exe,
		Meta:           parse.Meta,
		Logger:         r.Mgr,
		RuntimePort:    r.Mgr.RuntimePort,
		DBProxyPort:    r.Mgr.DBProxyPort,
		Secrets:        secrets,
		ServiceConfigs: build.Configs,
		Environ:        r.params.Environ,
		Experiments:    expSet,
	})
	if err != nil {
		tracker.Fail(startOp, err)
		return err
	}
	go func() {
		<-newProcess.Done()
		os.RemoveAll(build.Dir)
	}()

	previousProcess := r.proc.Swap(newProcess)
	if previousProcess != nil {
		previousProcess.(*Proc).Close()
	}

	tracker.Done(startOp, 50*time.Millisecond)

	return nil
}

// Proc represents a running Encore process.
type Proc struct {
	ID          string           // unique process id
	Run         *Run             // the run the process belongs to
	Pid         int              // the OS process id
	Meta        *meta.Data       // app metadata snapshot
	Started     time.Time        // when the process started
	Experiments *experiments.Set // enabled experiments

	ctx      context.Context
	log      zerolog.Logger
	exit     chan struct{} // closed when the process has exited
	cmd      *exec.Cmd
	reqWr    *os.File
	respRd   *os.File
	buildDir string
	Client   *yamux.Session
	authKey  config.EncoreAuthKey

	sym       *sym.Table
	symErr    error
	symParsed chan struct{} // closed when sym and symErr are set
}

type StartProcParams struct {
	Ctx            context.Context
	BuildDir       string
	BinPath        string
	Meta           *meta.Data
	Secrets        map[string]string
	ServiceConfigs map[string]string
	RuntimePort    int
	DBProxyPort    int
	Logger         RunLogger
	Environ        []string
	Experiments    *experiments.Set
}

// StartProc starts a single actual OS process for app.
func (r *Run) StartProc(params *StartProcParams) (p *Proc, err error) {
	pid := GenID()
	authKey := genAuthKey()
	p = &Proc{
		ID:          pid,
		Run:         r,
		Experiments: params.Experiments,
		Meta:        params.Meta,
		ctx:         params.Ctx,
		exit:        make(chan struct{}),
		buildDir:    params.BuildDir,
		log:         r.log.With().Str("proc_id", pid).Str("build_dir", params.BuildDir).Logger(),
		symParsed:   make(chan struct{}),
		authKey:     authKey,
	}
	go p.parseSymTable(params.BinPath)

	runtimeCfg, err := r.Mgr.generateConfig(generateConfigParams{
		App:         r.App,
		RM:          r.ResourceManager,
		Meta:        params.Meta,
		ForTests:    false,
		AuthKey:     authKey,
		APIBaseURL:  "http://" + r.ListenAddr,
		ConfigAppID: r.ID,
		ConfigEnvID: p.ID,
	})
	if err != nil {
		return nil, err
	}
	runtimeJSON, _ := json.Marshal(runtimeCfg)

	cmd := exec.Command(params.BinPath)
	envs := append(params.Environ,
		"ENCORE_RUNTIME_CONFIG="+base64.RawURLEncoding.EncodeToString(runtimeJSON),
		"ENCORE_APP_SECRETS="+encodeSecretsEnv(params.Secrets),
	)
	for serviceName, cfgString := range params.ServiceConfigs {
		envs = append(envs, "ENCORE_CFG_"+strings.ToUpper(serviceName)+"="+base64.RawURLEncoding.EncodeToString([]byte(cfgString)))
	}
	cmd.Env = envs
	p.cmd = cmd

	r.log.Info().RawJSON("config", runtimeJSON).Msgf("computed runtime config")

	// Proxy stdout and stderr to the given app logger, if any.
	if l := params.Logger; l != nil {
		cmd.Stdout = newLogWriter(r, l.RunStdout)
		cmd.Stderr = newLogWriter(r, l.RunStderr)
	}

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
		return nil, err
	} else if err := xos.ArrangeExtraFiles(cmd, reqRd, respWr); err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p.log.Info().Msg("started process")
	defer func() {
		if err != nil {
			cmd.Process.Kill()
		}
	}()

	// Close the files we handed over to the child.
	closeAll(reqRd, respWr)

	rwc := &struct {
		io.ReadCloser
		io.Writer
	}{
		ReadCloser: io.NopCloser(respRd),
		Writer:     reqWr,
	}
	p.Client, err = yamux.Client(rwc, yamux.DefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("could not initialize connection: %v", err)
	}

	p.reqWr = reqWr
	p.respRd = respRd
	p.Pid = cmd.Process.Pid
	p.Started = time.Now()

	// Monitor the context and Close the process when it is done.
	go func() {
		select {
		case <-params.Ctx.Done():
			p.Close()
		case <-p.exit:
		}
	}()

	go p.waitForExit()
	return p, nil
}

// Done returns a channel that is closed when the process has exited.
func (p *Proc) Done() <-chan struct{} {
	return p.exit
}

// Close closes the process and waits for it to shutdown.
// It can safely be called multiple times.
func (p *Proc) Close() {
	p.reqWr.Close()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-p.exit:
	case <-timer.C:
		// The process didn't exit after 10s
		p.log.Error().Msg("timed out waiting for process to exit; killing")
		p.cmd.Process.Kill()
		<-p.exit
	}
}

func (p *Proc) waitForExit() {
	defer close(p.exit)
	defer closeAll(p.reqWr, p.respRd)

	if err := p.cmd.Wait(); err != nil && p.ctx.Err() == nil {
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
}

// parseSymTable parses the symbol table of the binary at binPath
// and stores the result in p.sym and p.symErr.
func (p *Proc) parseSymTable(binPath string) {
	parse := func() (*sym.Table, error) {
		f, err := os.Open(binPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return sym.Load(f)
	}

	defer close(p.symParsed)
	p.sym, p.symErr = parse()
}

// SymTable waits for the proc's symbol table to be parsed and then returns it.
// ctx is used to cancel the wait.
func (p *Proc) SymTable(ctx context.Context) (*sym.Table, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.symParsed:
		return p.sym, p.symErr
	}
}

// closeAll closes all the given closers, skipping ones that are nil.
func closeAll(closers ...io.Closer) {
	for _, c := range closers {
		if c != nil {
			c.Close()
		}
	}
}

// firstErr reports the first non-nil error out of errs.
// If all are nil, it reports nil.
func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// logWriter is an io.Writer that buffers incoming logs
// and forwards whole log lines to fn.
type logWriter struct {
	run     *Run
	fn      func(r *Run, line []byte) // matches AppLogger.Log* signature
	maxLine int                       // max line length, including '\n'
	buf     *bytes.Buffer
}

func newLogWriter(run *Run, fn func(*Run, []byte)) *logWriter {
	const maxLine = 10 * 1024
	return &logWriter{
		run:     run,
		fn:      fn,
		maxLine: maxLine,
		buf:     bytes.NewBuffer(make([]byte, 0, maxLine)),
	}
}

func (w *logWriter) Write(b []byte) (int, error) {
	n := len(b)
	for {
		idx := bytes.IndexByte(b, '\n')
		if idx < 0 {
			break
		}
		// We have a line break; write the data to w.fn if it's not too long
		if (w.buf.Len() + idx + 1) <= w.maxLine {
			w.buf.Write(b[:idx+1])
			w.fn(w.run, w.buf.Bytes())
			w.buf.Reset()
		}
		b = b[idx+1:]
	}

	// Postcondition: we have some data remaining that doesn't contain a newline.
	// Write it to buf if it's not too long.
	if w.buf.Len()+len(b) <= w.maxLine {
		w.buf.Write(b)
	}
	return n, nil
}

// Flush flushes remaining data to w.fn along with a trailing newline.
// It must not be called concurrently with any writes to w.
func (w *logWriter) Flush() {
	if w.buf.Len() > 0 {
		w.buf.WriteByte('\n')
		w.fn(w.run, w.buf.Bytes())
		w.buf.Reset()
	}
}

// GenID generates a random run/process id.
// It panics if it cannot get random bytes.
func GenID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("cannot generate random data: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}

// encodeSecretsEnv encodes secrets to a value that can be passed in an env variable.
func encodeSecretsEnv(secrets map[string]string) string {
	if len(secrets) == 0 {
		return ""
	}

	// Sort the keys
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	first := true
	for _, k := range keys {
		if !first {
			buf.WriteByte(',')
		}
		first = false

		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(base64.RawURLEncoding.EncodeToString([]byte(secrets[k])))
	}
	return buf.String()
}

func usesSecrets(md *meta.Data) bool {
	for _, pkg := range md.Pkgs {
		if len(pkg.Secrets) > 0 {
			return true
		}
	}
	return false
}

func genAuthKey() config.EncoreAuthKey {
	kid := mathrand.Uint32()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("cannot generate random data: " + err.Error())
	}
	return config.EncoreAuthKey{KeyID: kid, Data: b[:]}
}
