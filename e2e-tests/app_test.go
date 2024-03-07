package tests

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gofrs/uuid"
	"github.com/rs/zerolog/log"

	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/cli/daemon/pubsub"
	"encr.dev/cli/daemon/redis"
	. "encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/run/infra"
	"encr.dev/cli/daemon/secret"
	. "encr.dev/internal/optracker"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/svcproxy"
	"encr.dev/pkg/vcs"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

type RunAppData struct {
	Addr  string
	Run   *Run
	Meta  *meta.Data
	NSQ   *pubsub.NSQDaemon
	Redis *redis.Server
	Env   []string

	Values map[string]any // arbitrary values for use in testscripts
}

func RunApp(c testing.TB, appRoot string, logger RunLogger, env []string) *RunAppData {
	assertNil := func(err error) {
		if err != nil {
			c.Fatal(err)
		}
	}

	ln, err := net.Listen("tcp", "localhost:0")
	assertNil(err)
	c.Cleanup(func() { _ = ln.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	c.Cleanup(cancel)

	svcProxy, err := svcproxy.New(ctx, log.Logger)
	assertNil(err)

	// Use a randomly generated app id to avoid tests trampling on each other
	// since we use a persistent working directory based on the app id.
	app := apps.NewInstance(appRoot, uuid.Must(uuid.NewV4()).String(), "")

	mgr := &Manager{}
	ns := &namespace.Namespace{ID: "some-id", Name: "default"}
	rm := infra.NewResourceManager(app, mgr.ClusterMgr, ns, nil, 0, false)
	run := &Run{
		ID:              GenID(),
		ListenAddr:      ln.Addr().String(),
		SvcProxy:        svcProxy,
		App:             app,
		ResourceManager: rm,
		Mgr:             mgr,
	}

	parse, build := testBuild(c, appRoot, env)

	jobs := NewAsyncBuildJobs(ctx, app.PlatformOrLocalID(), nil)
	run.ResourceManager.StartRequiredServices(jobs, parse.Meta)
	c.Cleanup(rm.StopAll)

	assertNil(jobs.Wait())

	env = append(env, "FOO=bar", "BAR=baz")

	if logger == nil {
		logger = testRunLogger{c}
	}

	expSet, err := experiments.FromAppFileAndEnviron(nil, env)
	assertNil(err)

	secrets := secret.New()
	secretData, err := secrets.Load(app).Get(ctx, expSet)
	assertNil(err)

	p, err := run.StartProcGroup(&StartProcGroupParams{
		Ctx:            ctx,
		Outputs:        build.Outputs,
		Meta:           parse.Meta,
		Logger:         logger,
		Environ:        env,
		ServiceConfigs: build.Configs,
		Experiments:    expSet,
		Secrets:        secretData.Values,
	})
	assertNil(err)
	c.Cleanup(p.Close)
	run.StoreProc(p)
	for serviceName, config := range build.Configs {
		env = append(env, fmt.Sprintf("%s=%s", fmt.Sprintf("ENCORE_CFG_%s", strings.ToUpper(serviceName)), base64.RawURLEncoding.EncodeToString([]byte(config))))
	}

	// start proxying TCP requests to the running application
	startProxy(ctx, ln, http.HandlerFunc(p.ProxyReq))

	// wait for the service to come up
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 10 * time.Second
	err = backoff.Retry(func() error {
		w := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost/__encore/healthz", nil)
		assertNil(err)

		p.ProxyReq(w, req)

		if w.Result().StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status: %s", w.Result().Status)
		}

		return nil
	}, b)
	assertNil(err)

	return &RunAppData{
		Addr:   ln.Addr().String(),
		Run:    run,
		Meta:   parse.Meta,
		NSQ:    rm.GetPubSub(),
		Redis:  rm.GetRedis(),
		Env:    env,
		Values: make(map[string]any),
	}
}

func RunTests(c testing.TB, appRoot string, stdout, stderr io.Writer, environ []string) error {
	mgr := &Manager{
		Secret:     secret.New(),
		ClusterMgr: nil,
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.Cleanup(cancel)

	// Use a randomly generated app id to avoid tests trampling on each other
	// since we use a persistent working directory based on the app id.
	app := apps.NewInstance(appRoot, uuid.Must(uuid.NewV4()).String(), "")
	err := mgr.Test(ctx, TestParams{
		App:        app,
		WorkingDir: ".",
		Args:       []string{"./..."},
		Environ:    environ,
		Stdout:     stdout,
		Stderr:     stderr,
	})
	return err
}

// testRunLogger implements runLogger by calling t.Log.
type testRunLogger struct {
	log interface{ Log(args ...any) }
}

func (l testRunLogger) RunStdout(r *Run, line []byte) {
	line = bytes.TrimSuffix(line, []byte{'\n'})
	l.log.Log(string(line))
}

func (l testRunLogger) RunStderr(r *Run, line []byte) {
	line = bytes.TrimSuffix(line, []byte{'\n'})
	l.log.Log(string(line))
}

func startProxy(ctx context.Context, ln net.Listener, proxyHandler http.Handler) {
	srv := &http.Server{Handler: proxyHandler}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	go func() { _ = srv.Serve(ln) }()
}

// testBuild is a helper that compiles the app situated at appRoot
// and cleans up the build dir during test cleanup.
func testBuild(t testing.TB, appRoot string, env []string) (*builder.ParseResult, *builder.CompileResult) {
	expSet, err := experiments.FromAppFileAndEnviron(nil, env)
	if err != nil {
		t.Fatal(err)
	}

	bld := builderimpl.Resolve(expSet)
	defer fns.CloseIgnore(bld)
	ctx := context.Background()

	// Use a randomly generated app id to avoid tests trampling on each other
	// since we use a persistent working directory based on the app id.
	app := apps.NewInstance(appRoot, uuid.Must(uuid.NewV4()).String(), "")

	vcsRevision := vcs.GetRevision(app.Root())
	buildInfo := builder.BuildInfo{
		BuildTags:          builder.LocalBuildTags,
		CgoEnabled:         true,
		StaticLink:         false,
		Debug:              false,
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		KeepOutput:         false,
		Revision:           vcsRevision.Revision,
		UncommittedChanges: vcsRevision.Uncommitted,
	}

	parse, err := bld.Parse(ctx, builder.ParseParams{
		Build:       buildInfo,
		App:         app,
		Experiments: expSet,
		WorkingDir:  ".",
		ParseTests:  false,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = bld.GenUserFacing(ctx, builder.GenUserFacingParams{
		Build: buildInfo,
		App:   app,
		Parse: parse,
	})
	if err != nil {
		t.Fatal(err)
	}

	build, err := bld.Compile(ctx, builder.CompileParams{
		Build:       buildInfo,
		App:         app,
		Parse:       parse,
		OpTracker:   nil,
		Experiments: expSet,
		WorkingDir:  ".",
		CueMeta: &cueutil.Meta{
			APIBaseURL: "http://what?",
			EnvName:    "end_to_end_test",
			EnvType:    cueutil.EnvType_Development,
			CloudType:  cueutil.CloudType_Local,
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		for _, output := range build.Outputs {
			_ = os.RemoveAll(output.GetArtifactDir().ToIO())
		}
	})
	return parse, build
}

func runGoModTidy(dir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go mod tidy failed: %s", out)
	}
	return nil
}
