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
	"io/ioutil"
	mathrand "math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/mod/modfile"

	"github.com/hashicorp/yamux"

	"encr.dev/cli/daemon/internal/sym"
	"encr.dev/cli/daemon/runtime/config"
	"encr.dev/cli/internal/appfile"
	"encr.dev/cli/internal/env"
	"encr.dev/cli/internal/xos"
	"encr.dev/compiler"
	"encr.dev/parser"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// Run represents a running Encore application.
type Run struct {
	ID         string // unique ID for this instance of the running app
	AppID      string // unique identifier for the app
	AppSlug    string // the optional app slug, if linked to encore.dev
	Root       string // the filesystem path to the app root
	ListenAddr string // the address the app is listening on

	log     zerolog.Logger
	mgr     *Manager
	params  *StartParams
	ctx     context.Context // ctx is closed when the run is to exit
	proc    atomic.Value    // current process
	exited  chan struct{}   // exit is closed when the run has fully exited
	started chan struct{}   // started is closed once the run has fully started

}

// StartParams groups the parameters for the Run method.
type StartParams struct {
	// AppRoot is the application root.
	AppRoot string

	// AppID is the unique app id, as defined by the manifest.
	AppID string

	// WorkingDir is the working dir, for formatting
	// error messages with relative paths.
	WorkingDir string

	// DBClusterID is the database cluster id to connect to.
	DBClusterID string

	// Parse is the parse result for the initial run of the app.
	// If nil the app is parsed before starting.
	Parse *parser.Result

	// Watch enables watching for code changes for live reloading.
	Watch bool

	Listener   net.Listener // listener to use
	ListenAddr string       // address we're listening on

	// Environ are the environment variables to set for the running app,
	// in the same format as os.Environ().
	Environ []string
}

// Start starts the application.
// Its lifetime is bounded by ctx.
func (mgr *Manager) Start(ctx context.Context, params StartParams) (run *Run, err error) {
	appSlug, err := appfile.Slug(params.AppRoot)
	if err != nil {
		return nil, err
	}

	run = &Run{
		ID:         genID(),
		AppID:      params.AppID,
		Root:       params.AppRoot,
		AppSlug:    appSlug,
		ListenAddr: params.ListenAddr,

		log:     log.With().Str("appID", params.AppID).Logger(),
		mgr:     mgr,
		params:  &params,
		ctx:     ctx,
		exited:  make(chan struct{}),
		started: make(chan struct{}),
	}

	// Add the run to our map before starting to avoid
	// racing with initialization (though it's unlikely to ever matter).
	mgr.mu.Lock()
	if mgr.runs == nil {
		mgr.runs = make(map[string]*Run)
	}
	mgr.runs[run.ID] = run
	mgr.mu.Unlock()

	if err := run.start(params.Listener); err != nil {
		return nil, err
	}

	if params.Watch {
		if err := mgr.watch(run); err != nil {
			return nil, err
		}
	}

	return run, nil
}

// runLogger is the interface for listening to run logs.
// The log methods are called for each logline on stdout and stderr respectively.
type runLogger interface {
	runStdout(r *Run, line []byte)
	runStderr(r *Run, line []byte)
}

// Proc returns the current running process.
// It may have already exited.
// If the proc has not yet started it may return nil.
func (r *Run) Proc() *Proc {
	p, _ := r.proc.Load().(*Proc)
	return p
}

// Done returns a channel that is closed when the run is closed.
func (r *Run) Done() <-chan struct{} {
	return r.exited
}

// Reload rebuilds the app and, if successful,
// starts a new proc and switches over.
func (r *Run) Reload() (*Proc, error) {
	modPath := filepath.Join(r.Root, "go.mod")
	modData, err := ioutil.ReadFile(modPath)
	if err != nil {
		return nil, err
	}
	mod, err := modfile.Parse(modPath, modData, nil)
	if err != nil {
		return nil, err
	}

	cfg := &parser.Config{
		AppRoot:    r.Root,
		Version:    "",
		ModulePath: mod.Module.Mod.Path,
		WorkingDir: r.params.WorkingDir,
		ParseTests: false,
	}
	parse, err := parser.Parse(cfg)
	if err != nil {
		return nil, err
	}

	p, err := r.buildAndStart(r.ctx, parse)
	if err != nil {
		return nil, err
	}

	prev := r.Proc()
	r.proc.Store(p)
	prev.close()

	for _, ln := range r.mgr.listeners {
		ln.OnReload(r)
	}

	return p, nil
}

// start starts the application and serves requests over HTTP using ln.
func (r *Run) start(ln net.Listener) (err error) {
	defer func() {
		if err != nil {
			// This is closed below when err == nil,
			// so handle the other cases.
			close(r.started)
			close(r.exited)
		}
	}()

	p, err := r.buildAndStart(r.ctx, r.params.Parse)
	if err != nil {
		return err
	}
	r.proc.Store(p)

	// Below this line the function must never return an error
	// in order to only ensure we close r.exited exactly once.

	go func() {
		for _, ln := range r.mgr.listeners {
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

	// Monitor the running proc and close the app when it exits.
	go func() {
		for {
			p := r.proc.Load().(*Proc)
			<-p.Done()
			// p exited, but it could have been a reload.
			// Check to make sure p is still the active proc.
			p2 := r.proc.Load().(*Proc)
			if p2 == p {
				// We're done.
				for _, ln := range r.mgr.listeners {
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
func (r *Run) buildAndStart(ctx context.Context, parse *parser.Result) (p *Proc, err error) {
	// Return early if the ctx is already canceled.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cfg := &compiler.Config{
		Version:           "",
		WorkingDir:        r.params.WorkingDir,
		CgoEnabled:        true,
		EncoreRuntimePath: env.EncoreRuntimePath(),
		EncoreGoRoot:      env.EncoreGoRoot(),
		Parse:             parse,
		BuildTags:         []string{"encore_local"},
	}

	build, err := compiler.Build(r.Root, cfg)
	if err != nil {
		return nil, fmt.Errorf("compile error:\n%v", err)
	}
	defer func() {
		if err != nil {
			os.RemoveAll(build.Dir)
		}
	}()

	var secrets map[string]string
	if usesSecrets(r.params.Parse.Meta) {
		if r.AppSlug == "" {
			return nil, fmt.Errorf("the app defines secrets, but is not yet linked to encore.dev; link it with `encore app link` to use secrets")
		}
		data, err := r.mgr.Secret.Get(ctx, r.AppSlug)
		if err != nil {
			return nil, err
		}
		secrets = data.Values
	}

	p, err = r.startProc(&startProcParams{
		Ctx:         ctx,
		BuildDir:    build.Dir,
		BinPath:     build.Exe,
		Meta:        build.Parse.Meta,
		Logger:      r.mgr,
		RuntimePort: r.mgr.RuntimePort,
		DBProxyPort: r.mgr.DBProxyPort,
		DBClusterID: r.params.DBClusterID,
		Secrets:     secrets,
		Environ:     r.params.Environ,
	})
	if err != nil {
		return nil, err
	}
	go func() {
		<-p.Done()
		os.RemoveAll(build.Dir)
	}()
	return p, nil
}

// Proc represents a running Encore process.
type Proc struct {
	ID      string     // unique process id
	Run     *Run       // the run the process belongs to
	Pid     int        // the OS process id
	Meta    *meta.Data // app metadata snapshot
	Started time.Time  // when the process started

	log      zerolog.Logger
	exit     chan struct{} // closed when the process has exited
	cmd      *exec.Cmd
	reqWr    *os.File
	respRd   *os.File
	buildDir string
	client   *yamux.Session
	authKey  config.EncoreAuthKey

	sym       *sym.Table
	symErr    error
	symParsed chan struct{} // closed when sym and symErr are set
}

type startProcParams struct {
	Ctx         context.Context
	BuildDir    string
	BinPath     string
	Meta        *meta.Data
	Secrets     map[string]string
	RuntimePort int
	DBProxyPort int
	DBClusterID string
	Logger      runLogger
	Environ     []string
}

// startProc starts a single actual OS process for app.
func (r *Run) startProc(params *startProcParams) (p *Proc, err error) {
	pid := genID()
	authKey := genAuthKey()
	p = &Proc{
		ID:        pid,
		Run:       r,
		Meta:      params.Meta,
		exit:      make(chan struct{}),
		buildDir:  params.BuildDir,
		log:       r.log.With().Str("procID", pid).Str("buildDir", params.BuildDir).Logger(),
		symParsed: make(chan struct{}),
		authKey:   authKey,
	}
	go p.parseSymTable(params.BinPath)

	runtimeCfg := r.generateConfig(p, params)
	runtimeJSON, _ := json.Marshal(runtimeCfg)

	cmd := exec.Command(params.BinPath)
	cmd.Env = append(params.Environ,
		"ENCORE_RUNTIME_CONFIG="+base64.RawURLEncoding.EncodeToString(runtimeJSON),
		"ENCORE_APP_SECRETS="+encodeSecretsEnv(params.Secrets),
	)
	p.cmd = cmd

	// Proxy stdout and stderr to the given app logger, if any.
	if l := params.Logger; l != nil {
		cmd.Stdout = newLogWriter(r, l.runStdout)
		cmd.Stderr = newLogWriter(r, l.runStderr)
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
		ReadCloser: ioutil.NopCloser(respRd),
		Writer:     reqWr,
	}
	p.client, err = yamux.Client(rwc, yamux.DefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("could not initialize connection: %v", err)
	}

	p.reqWr = reqWr
	p.respRd = respRd
	p.Pid = cmd.Process.Pid
	p.Started = time.Now()

	// Monitor the context and close the process when it is done.
	go func() {
		select {
		case <-params.Ctx.Done():
			p.close()
		case <-p.exit:
		}
	}()

	go p.waitForExit()
	return p, nil
}

func (r *Run) generateConfig(p *Proc, params *startProcParams) *config.Runtime {
	var dbs []*config.SQLDatabase
	for _, svc := range params.Meta.Svcs {
		if len(svc.Migrations) > 0 {
			dbs = append(dbs, &config.SQLDatabase{
				EncoreName:   svc.Name,
				DatabaseName: svc.Name,
				Host:         "localhost:" + strconv.Itoa(params.DBProxyPort),
				User:         "encore",
				Password:     params.DBClusterID,
			})
		}
	}
	return &config.Runtime{
		AppID:         r.ID,
		AppSlug:       r.AppSlug,
		APIBaseURL:    "http://" + r.ListenAddr,
		EnvID:         p.ID,
		EnvName:       "local",
		EnvType:       "local",
		TraceEndpoint: "http://localhost:" + strconv.Itoa(params.RuntimePort) + "/trace",
		SQLDatabases:  dbs,
		AuthKeys:      []config.EncoreAuthKey{p.authKey},
	}
}

// Done returns a channel that is closed when the process has exited.
func (p *Proc) Done() <-chan struct{} {
	return p.exit
}

// close closes the process and waits for it to shutdown.
// It can safely be called multiple times.
func (p *Proc) close() {
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

	if err := p.cmd.Wait(); err != nil {
		p.log.Error().Err(err).Msg("process exited with error")
	} else {
		p.log.Info().Err(err).Msg("process exited successfully")
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

// genID generates a random run/process id.
// It panics if it cannot get random bytes.
func genID() string {
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
