// Package run starts and tracks running Encore applications.
package run

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/logrusorgru/aurora/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/cli/daemon/run/infra"
	"encr.dev/cli/daemon/secret"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/option"
	"encr.dev/pkg/svcproxy"
	"encr.dev/pkg/vcs"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// Run represents a running Encore application.
type Run struct {
	ID              string // unique ID for this instance of the running app
	App             *apps.Instance
	ListenAddr      string // the address the app is listening on
	SvcProxy        *svcproxy.SvcProxy
	ResourceManager *infra.ResourceManager
	NS              *namespace.Namespace

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

	// NS is the namespace to use.
	NS *namespace.Namespace

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
	logger := log.With().Str("app_id", params.App.PlatformOrLocalID()).Logger()

	svcProxy, err := svcproxy.New(ctx, logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create service proxy")
	}

	run = &Run{
		ID:              GenID(),
		App:             params.App,
		NS:              params.NS,
		ResourceManager: infra.NewResourceManager(params.App, mgr.ClusterMgr, params.NS, params.Environ, mgr.DBProxyPort, false),
		ListenAddr:      params.ListenAddr,
		SvcProxy:        svcProxy,
		log:             logger,
		Mgr:             mgr,
		params:          &params,
		secrets:         mgr.Secret.Load(params.App),
		ctx:             ctx,
		exited:          make(chan struct{}),
		started:         make(chan struct{}),
	}
	defer func(r *Run) {
		// Stop all the resource servers if we exit due to an error
		if err != nil {
			r.Close()
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
		if errList := AsErrorList(err); errList != nil {
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

func (r *Run) Close() {
	r.SvcProxy.Close()
	r.ResourceManager.StopAll()
}

// RunLogger is the interface for listening to run logs.
// The log methods are called for each logline on stdout and stderr respectively.
type RunLogger interface {
	RunStdout(r *Run, line []byte)
	RunStderr(r *Run, line []byte)
}

// ProcGroup returns the current running process.
// It may have already exited.
// If the proc has not yet started it may return nil.
func (r *Run) ProcGroup() *ProcGroup {
	p, _ := r.proc.Load().(*ProcGroup)
	return p
}

func (r *Run) StoreProc(p *ProcGroup) {
	r.proc.Store(p)
}

// Done returns a channel that is closed when the run is closed.
func (r *Run) Done() <-chan struct{} {
	return r.exited
}

// Reload rebuilds the app and, if successful,
// starts a new proc and switches over.
func (r *Run) Reload() error {
	err := r.buildAndStart(r.ctx, nil, true)
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

	err = r.buildAndStart(r.ctx, tracker, false)
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
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			r.log.Error().Err(err).Msg("could not serve")
		}
	}()
	go func() {
		<-r.ctx.Done()
		_ = srv.Close()
	}()

	// Monitor the running proc and Close the app when it exits.
	go func() {
		for {
			p := r.proc.Load().(*ProcGroup)
			<-p.Done()
			// p exited, but it could have been a reload.
			// Check to make sure p is still the active proc.
			p2 := r.proc.Load().(*ProcGroup)
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
func (r *Run) buildAndStart(ctx context.Context, tracker *optracker.OpTracker, isReload bool) error {
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
	newProcess, err := r.StartProcGroup(&StartProcGroupParams{
		Ctx:            ctx,
		BuildDir:       build.Dir,
		BinPath:        build.Exe,
		Meta:           parse.Meta,
		Logger:         r.Mgr,
		Secrets:        secrets,
		ServiceConfigs: build.Configs,
		Environ:        r.params.Environ,
		WorkingDir:     r.params.WorkingDir,
		IsReload:       isReload,
		Experiments:    expSet,
	})
	if err != nil {
		tracker.Fail(startOp, err)
		return err
	}

	previousProcess := r.proc.Swap(newProcess)
	if previousProcess != nil {
		previousProcess.(*ProcGroup).Close()
	}

	tracker.Done(startOp, 50*time.Millisecond)

	go func() {
		// Wait one second before logging all the missing secrets.
		time.Sleep(1 * time.Second)

		// Log any warnings.
		for _, warning := range newProcess.Warnings() {
			line := "\n" + aurora.Red(fmt.Sprintf("warning: %s", warning.Title)).String() + "\n" +
				aurora.Gray(16, fmt.Sprintf("note: %s", warning.Help)).String() + "\n\n"
			r.Mgr.RunStderr(r, []byte(line))
		}
	}()

	return nil
}

type StartProcGroupParams struct {
	Ctx            context.Context
	BuildDir       string
	BinPath        string
	Meta           *meta.Data
	Secrets        map[string]string
	ServiceConfigs map[string]string
	Logger         RunLogger
	Environ        []string
	WorkingDir     string
	IsReload       bool
	Experiments    *experiments.Set
}

const gracefulShutdownTime = 10 * time.Second

// StartProcGroup starts a single actual OS process for app.
func (r *Run) StartProcGroup(params *StartProcGroupParams) (p *ProcGroup, err error) {
	pid := GenID()
	authKey := genAuthKey()

	procListenAddresses, err := GenerateListenAddresses(r.SvcProxy, params.Meta.Svcs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate listen addresses")
	}

	listenAddr, err := netip.ParseAddrPort(strings.ReplaceAll(r.ListenAddr, "localhost", "127.0.0.1"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse listen address: %s", r.ListenAddr)
	}

	p = &ProcGroup{
		ID:  pid,
		Run: r,
		EnvGenerator: &RuntimeEnvGenerator{
			App:                  r.App,
			InfraManager:         r.ResourceManager,
			Meta:                 params.Meta,
			Secrets:              params.Secrets,
			SvcConfigs:           params.ServiceConfigs,
			AppID:                option.Some(r.ID),
			EnvID:                option.Some(pid),
			TraceEndpoint:        option.Some(fmt.Sprintf("http://localhost:%d/trace", r.Mgr.RuntimePort)),
			AuthKey:              option.Some(authKey),
			DaemonProxyAddr:      option.Some(listenAddr),
			GracefulShutdownTime: option.Some(gracefulShutdownTime),
			ShutdownHooksGrace:   option.Some(4 * time.Second),
			HandlersGrace:        option.Some(2 * time.Second),
			ListenAddresses:      procListenAddresses,
		},
		Experiments: params.Experiments,
		Meta:        params.Meta,
		ctx:         params.Ctx,
		buildDir:    params.BuildDir,
		workingDir:  params.WorkingDir,
		logger:      params.Logger,
		log:         r.log.With().Str("proc_id", pid).Str("build_dir", params.BuildDir).Logger(),
		symParsed:   make(chan struct{}),
		authKey:     authKey,
		Services:    make(map[string]*Proc),
	}
	p.procCond.L = &p.procMu
	go p.parseSymTable(params.BinPath)

	newProcParams := &NewProcParams{
		BinPath: params.BinPath,
		Environ: params.Environ,
	}

	// If we're testing external calls, start a process for each service.
	if experiments.LocalMultiProcess.Enabled(params.Experiments) {
		// create a process for each service
		for _, s := range params.Meta.Svcs {
			if err := p.NewProcForService(
				s,
				procListenAddresses.Services[s.Name].ListenAddr,
				newProcParams,
			); err != nil {
				return nil, err
			}
		}

		// create a process for the gateway
		if err := p.NewProcForGateway(procListenAddresses.Gateway.ListenAddr, newProcParams); err != nil {
			return nil, err
		}
	} else {
		// If we're not testing external calls, we only need a single process
		// so no need to pass in the listen addresses.
		procListenAddresses.Services = nil

		// Otherwise we're running everything inside a single process
		if err := p.NewAllInOneProc(newProcParams); err != nil {
			return nil, err
		}
	}

	// Start the processes of the application
	if err := p.Start(); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			p.Kill()
		}
	}()

	// Monitor the context and Close the process when it is done.
	go func() {
		select {
		case <-params.Ctx.Done():
			p.Close()
		case <-p.Done():
		}
	}()

	// If this is a live reload, wait for the process to be ready.
	// This way we ensure requests are always hitting a running server,
	// in case a batch job or something is running.
	if params.IsReload {
		p.Gateway.pollUntilProcessIsListening(params.Ctx)
	}

	return p, nil
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
	// read a uint32 from crypto/rand to use as the key ID
	var kidBytes [4]byte
	if _, err := rand.Read(kidBytes[:]); err != nil {
		panic("cannot generate random data: " + err.Error())
	}
	kid := binary.BigEndian.Uint32(kidBytes[:])

	// kid := mathrand.Uint32()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("cannot generate random data: " + err.Error())
	}
	return config.EncoreAuthKey{KeyID: kid, Data: b[:]}
}

// CanDeleteNamespace implements namespace.DeletionHandler.
func (m *Manager) CanDeleteNamespace(ctx context.Context, app *apps.Instance, ns *namespace.Namespace) error {
	// Check if any of the active runs are using this namespace.
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.runs {
		if r.NS.ID == ns.ID && r.ctx.Err() == nil {
			return errors.New("namespace is in use by 'encore run'")
		}
	}
	return nil
}

// DeleteNamespace implements namespace.DeletionHandler.
func (m *Manager) DeleteNamespace(ctx context.Context, app *apps.Instance, ns *namespace.Namespace) error {
	// We don't need to do anything here; we only implement DeletionHandler for
	// the CanDeleteNamespace check.
	return nil
}
