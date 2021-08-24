package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"encr.dev/cli/daemon/internal/appfile"
	"encr.dev/cli/daemon/internal/manifest"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/cli/internal/onboarding"
	"encr.dev/cli/internal/version"
	"encr.dev/parser"
	daemonpb "encr.dev/proto/encore/daemon"
	meta "encr.dev/proto/encore/parser/meta/v1"
	"github.com/logrusorgru/aurora/v3"
	"github.com/rs/zerolog/log"
	"golang.org/x/mod/modfile"
)

// Run runs the application.
func (s *Server) Run(req *daemonpb.RunRequest, stream daemonpb.Daemon_RunServer) error {
	slog := &streamLog{stream: stream, buffered: true}
	stderr := slog.Stderr(false)

	sendExit := func(code int32) {
		stream.Send(&daemonpb.CommandMessage{
			Msg: &daemonpb.CommandMessage_Exit{Exit: &daemonpb.CommandExit{
				Code: code,
			}},
		})
	}

	// Prefetch secrets if the app is linked.
	if appSlug, err := appfile.Slug(req.AppRoot); err == nil && appSlug != "" {
		s.sm.Prefetch(appSlug)
	}

	// Clear screen.
	stderr.Write([]byte("\033[2J\033[H\n"))

	ops := newOpTracker(stderr)

	// Parse the app to figure out what infrastructure is needed.
	start := time.Now()
	parseOp := ops.Add("Building Encore application graph", start)
	topoOp := ops.Add("Analyzing service topology", start.Add(450*time.Millisecond))
	parse, err := s.parseApp(req.AppRoot, req.WorkingDir, false)
	if err != nil {
		ops.Fail(parseOp, err)
		sendExit(1)
		return nil
	}
	man, err := manifest.ReadOrCreate(req.AppRoot)
	if err != nil {
		ops.Fail(parseOp, err)
		sendExit(1)
		return nil
	}
	s.cacheAppRoot(man.AppID, req.AppRoot)
	ops.Done(parseOp, 500*time.Millisecond)
	ops.Done(topoOp, 300*time.Millisecond)

	clusterID := man.AppID
	dbSetupCh := make(chan error, 1)

	// Set up the database only if the app requires it.
	if requiresSQLDB(parse.Meta) {
		dbOp := ops.Add("Creating PostgreSQL database cluster", start.Add(300*time.Millisecond))
		cluster := s.cm.Init(stream.Context(), &sqldb.InitParams{
			ClusterID: clusterID,
			Meta:      parse.Meta,
			Memfs:     false,
		})
		if _, err := exec.LookPath("docker"); err != nil {
			ops.Fail(dbOp, errors.New("This application requires docker to run since it uses an SQL database. Install docker first."))
			sendExit(1)
			return nil
		}

		if ok, err := sqldb.ImageExists(stream.Context()); err == nil && !ok {
			pullOp := ops.Add("Pulling PostgreSQL docker image", start.Add(400*time.Millisecond))
			if err := sqldb.PullImage(stream.Context()); err != nil {
				ops.Fail(pullOp, err)
			} else {
				ops.Done(pullOp, 0)
			}
		}

		if err := cluster.Start(slog); err != nil {
			ops.Fail(dbOp, err)
			sendExit(1)
			return nil
		}
		ops.Done(dbOp, 700*time.Millisecond)

		// Set up the database asynchronously since it can take a while.
		migrateOp := ops.Add("Running database migrations", start.Add(700*time.Millisecond))
		go func() {
			err := cluster.CreateAndMigrate(stream.Context(), req.AppRoot, parse.Meta)
			if err != nil {
				log.Error().Err(err).Msg("failed to setup db")
				ops.Fail(migrateOp, err)
			} else {
				ops.Done(migrateOp, 500*time.Millisecond)
			}
			dbSetupCh <- err
		}()
	} else {
		dbSetupCh <- nil // no database to set up
	}

	// Check for available update before we start the proc
	// so the output from the proc doesn't race with our
	// prints below.
	newVer := s.availableUpdate()

	codegenOp := ops.Add("Generating boilerplate code", start.Add(1000*time.Millisecond))
	compileOp := ops.Add("Compiling application source code", start.Add(1500*time.Millisecond))
	ops.Done(codegenOp, 450*time.Millisecond)

	// Hold the stream mutex so we can set up the stream map
	// before output starts.
	s.mu.Lock()
	run, err := s.mgr.Start(stream.Context(), run.StartParams{
		AppRoot:     req.AppRoot,
		AppID:       man.AppID,
		WorkingDir:  req.WorkingDir,
		DBClusterID: clusterID,
		Parse:       parse,
		Watch:       req.Watch,
	})
	if err != nil {
		s.mu.Unlock()
		ops.Fail(compileOp, err)
		sendExit(1)
		return nil
	}
	ops.Done(compileOp, 300*time.Millisecond)
	s.streams[run.ID] = slog
	s.mu.Unlock()

	if err := <-dbSetupCh; err != nil {
		sendExit(1)
		return nil
	}

	ops.AllDone()

	stderr.Write([]byte("\n"))
	pid := run.Proc().Pid
	fmt.Fprintf(stderr, "  Encore development server running!\n\n")

	fmt.Fprintf(stderr, "  Your API is running at:     %s\n", aurora.Cyan(fmt.Sprintf("http://localhost:%d", run.Port)))
	fmt.Fprintf(stderr, "  Development Dashboard URL:  %s\n", aurora.Cyan(fmt.Sprintf("http://localhost:%d/%s", s.mgr.DashPort, man.AppID)))
	if req.Debug {
		fmt.Fprintf(stderr, "  Process ID:             %d\n", aurora.Cyan(pid))
	}
	if newVer != "" {
		stderr.Write([]byte(aurora.Sprintf(
			aurora.Faint("\n  New Encore release available: %s (you have %s)\n  Update with: encore version update\n"),
			newVer, version.Version)))
	}
	stderr.Write([]byte("\n"))

	slog.FlushBuffers()

	go func() {
		// Wait a little bit for the app to start
		select {
		case <-run.Done():
			return
		case <-time.After(5 * time.Second):
			showFirstRunExperience(run, parse.Meta, stderr)
		}
	}()

	<-run.Done() // wait for run to complete

	s.mu.Lock()
	delete(s.streams, run.ID)
	s.mu.Unlock()
	return nil
}

// Test runs tests.
func (s *Server) Test(req *daemonpb.TestRequest, stream daemonpb.Daemon_TestServer) error {
	slog := &streamLog{stream: stream, buffered: false}
	sendErr := func(err error) {
		slog.Stderr(false).Write([]byte(err.Error() + "\n"))
		streamExit(stream, 1)
	}

	// Prefetch secrets if the app is linked.
	if appSlug, err := appfile.Slug(req.AppRoot); err == nil && appSlug != "" {
		s.sm.Prefetch(appSlug)
	}

	// Parse the app to figure out what infrastructure is needed.
	parse, err := s.parseApp(req.AppRoot, req.WorkingDir, true /* parse tests */)
	if err != nil {
		sendErr(err)
		return nil
	}

	man, err := manifest.ReadOrCreate(req.AppRoot)
	if err != nil {
		sendErr(err)
		return nil
	}
	s.cacheAppRoot(man.AppID, req.AppRoot)

	setupCtx, setupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	clusterID := man.AppID + "-test"
	cluster := s.cm.Init(setupCtx, &sqldb.InitParams{
		ClusterID: clusterID,
		Memfs:     true,
		Meta:      parse.Meta,
	})

	// Set up the database asynchronously since it can take a while.
	dbSetupErr := make(chan error, 1)
	go func() {
		defer setupCancel()
		if err := cluster.Start(&streamLog{stream: stream, buffered: false}); err != nil {
			dbSetupErr <- err
		} else if err := cluster.Recreate(setupCtx, req.AppRoot, nil, parse.Meta); err != nil {
			dbSetupErr <- err
		}
	}()

	testCtx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	testResults := make(chan error, 1)
	go func() {
		testResults <- s.mgr.Test(testCtx, run.TestParams{
			AppRoot:     req.AppRoot,
			WorkingDir:  req.WorkingDir,
			DBClusterID: clusterID,
			Args:        req.Args,
			Stdout:      slog.Stdout(false),
			Stderr:      slog.Stderr(false),
		})
	}()

	select {
	case err := <-dbSetupErr:
		sendErr(err)
		return nil
	case err := <-testResults:
		if err != nil {
			sendErr(err)
		} else {
			streamExit(stream, 0)
		}
		return nil
	}
}

// Check checks the app for compilation errors.
func (s *Server) Check(req *daemonpb.CheckRequest, stream daemonpb.Daemon_CheckServer) error {
	slog := &streamLog{stream: stream, buffered: false}
	log := newStreamLogger(slog)
	err := s.mgr.Check(stream.Context(), req.AppRoot, req.WorkingDir)
	if err != nil {
		log.Error().Msg(err.Error())
		streamExit(stream, 1)
	} else {
		streamExit(stream, 0)
	}
	return nil
}

// OnStart implements run.EventListener.
func (s *Server) OnStart(r *run.Run) {}

// OnReload implements run.EventListener.
func (s *Server) OnReload(r *run.Run) {}

// OnStop implements run.EventListener.
func (s *Server) OnStop(r *run.Run) {}

// OnStdout implements run.EventListener.
func (s *Server) OnStdout(r *run.Run, line []byte) {
	s.mu.Lock()
	slog, ok := s.streams[r.ID]
	s.mu.Unlock()

	if ok {
		slog.Stdout(true).Write(line)
	}
}

// OnStderr implements run.EventListener.
func (s *Server) OnStderr(r *run.Run, line []byte) {
	s.mu.Lock()
	slog, ok := s.streams[r.ID]
	s.mu.Unlock()

	if ok {
		slog.Stderr(true).Write(line)
	}
}

// parseApp parses the app.
func (s *Server) parseApp(appRoot, workingDir string, parseTests bool) (*parser.Result, error) {
	modPath := filepath.Join(appRoot, "go.mod")
	modData, err := ioutil.ReadFile(modPath)
	if err != nil {
		return nil, err
	}
	mod, err := modfile.Parse(modPath, modData, nil)
	if err != nil {
		return nil, err
	}

	cfg := &parser.Config{
		AppRoot:    appRoot,
		Version:    "",
		ModulePath: mod.Module.Mod.Path,
		WorkingDir: workingDir,
		ParseTests: parseTests,
	}
	return parser.Parse(cfg)
}

func requiresSQLDB(md *meta.Data) bool {
	for _, svc := range md.Svcs {
		if len(svc.Migrations) > 0 {
			return true
		}
	}
	return false
}

func showFirstRunExperience(run *run.Run, md *meta.Data, stdout io.Writer) {
	if state, err := onboarding.Load(); err == nil {
		if !state.FirstRun.IsSet() {
			// Is there a suitable endpoint to call?
			var rpc *meta.RPC
			var command string
			for _, svc := range md.Svcs {
				for _, r := range svc.Rpcs {
					if cmd := genCurlCommand(run, md, r); rpc == nil || len(command) < len(cmd) {
						rpc = r
						command = cmd
					}
				}
			}
			if rpc != nil {
				state.FirstRun.Set()
				if err := state.Write(); err == nil {
					stdout.Write([]byte(aurora.Sprintf("\nHint: make an API call by running: %s\n", aurora.Cyan(command))))
				}
			}
		}
	}
}

func genCurlCommand(run *run.Run, md *meta.Data, rpc *meta.RPC) string {
	payload := genSchema(md, rpc.RequestSchema)
	var segments []string
	for _, seg := range rpc.Path.Segments {
		var v string
		switch seg.Type {
		default:
			v = "foo"
		case meta.PathSegment_LITERAL:
			v = seg.Value
		case meta.PathSegment_WILDCARD:
			v = "foo"
		case meta.PathSegment_PARAM:
			switch seg.ValueType {
			case meta.PathSegment_STRING:
				v = "foo"
			case meta.PathSegment_BOOL:
				v = "true"
			case meta.PathSegment_INT8, meta.PathSegment_INT16, meta.PathSegment_INT32, meta.PathSegment_INT64,
				meta.PathSegment_UINT8, meta.PathSegment_UINT16, meta.PathSegment_UINT32, meta.PathSegment_UINT64:
				v = "1"
			case meta.PathSegment_UUID:
				v = "be23a21f-d12c-432c-91ec-fb8a52e23967" // some random UUID
			default:
				v = "foo"
			}
		}
		segments = append(segments, v)
	}
	method := rpc.HttpMethods[0]
	parts := []string{"curl"}
	if (payload != nil && method != "POST") || (payload == nil && method != "GET") {
		parts = append(parts, " -X ", method)
	}
	path := "/" + strings.Join(segments, "/")
	parts = append(parts, " http://localhost:", strconv.Itoa(run.Port), path)
	if payload != nil {
		parts = append(parts, " -d '", string(payload), "'")
	}
	return strings.Join(parts, "")
}

func newOpTracker(w io.Writer) *opTracker {
	return &opTracker{
		w: w,
	}
}

type opTracker struct {
	mu      sync.Mutex
	ops     []*slowOp
	w       io.Writer
	nl      int // number of lines written
	started bool
	quit    bool
}

// AllDone marks all ops as done.
func (t *opTracker) AllDone() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	for _, o := range t.ops {
		if o.done.IsZero() || o.done.After(now) {
			o.done = now
		}
		if o.start.After(now) {
			o.start = now
		}
	}
	t.quit = true
	t.refresh()
}

func (t *opTracker) Add(msg string, minStart time.Time) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	id := len(t.ops)

	start := time.Now()
	if start.Before(minStart) {
		start = minStart
	}
	op := &slowOp{msg: msg, start: start}
	t.ops = append(t.ops, op)
	t.refresh()

	if !t.started {
		go t.spin()
		t.started = true
	}

	return id
}

func (t *opTracker) Done(id int, minDuration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	o := t.ops[id]

	done := time.Now()
	if a := o.start.Add(minDuration); a.After(done) {
		done = a
	}
	o.done = done
	t.refresh()
}

func (t *opTracker) Fail(id int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ops[id].done.IsZero() {
		return
	}
	t.ops[id].err = err
	t.ops[id].done = time.Now()
	t.refresh()
}

// refresh refreshes the display by writing to t.w.
// The mutex must be held by the caller.
func (t *opTracker) refresh() {
	fmt.Fprint(t.w, "\u001b[0;0H\u001b[0J\n")

	nl := 0
	now := time.Now()

	// Sort ops by start time
	ops := make([]*slowOp, len(t.ops))
	copy(ops, t.ops)
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].start.Before(ops[j].start)
	})

	for _, o := range ops {
		started := o.start.Before(now)
		done := o.done.Before(now)
		if !started && !done {
			continue
		}

		var msg aurora.Value
		format := "  %s %s... "
		switch {
		case done && o.err != nil:
			msg = aurora.Red(fmt.Sprintf(format+"Failed: %v", fail, o.msg, o.err))
		case done && o.err == nil:
			msg = aurora.Green(fmt.Sprintf(format+"Done!", success, o.msg))
		case !done:
			msg = aurora.Cyan(fmt.Sprintf(format, spinner[o.spinIdx], o.msg))
			o.spinIdx = (o.spinIdx + 1) % len(spinner)
		}
		str := msg.String()
		fmt.Fprintf(t.w, "\u001b[2K%s\n", str)
		nl += strings.Count(str, "\n") + 1
	}
	t.nl = nl
}

func (t *opTracker) spin() {
	refresh := 100 * time.Millisecond
	if runtime.GOOS == "windows" {
		// Window's terminal is quite slow at rendering.
		// Reduce the refresh rate to avoid excessive flickering.
		refresh = 250 * time.Millisecond
	}
	for {
		time.Sleep(refresh)
		(func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			if !t.quit {
				t.refresh()
			}
		})()
	}

}

type slowOp struct {
	msg     string
	err     error
	spinIdx int
	start   time.Time
	done    time.Time
}

var (
	success = "✔"
	fail    = "❌"
	spinner = []string{"⠋", "⠙", "⠚", "⠒", "⠂", "⠂", "⠒", "⠲", "⠴", "⠦", "⠖", "⠒", "⠐", "⠐", "⠒", "⠓", "⠋"}
)
