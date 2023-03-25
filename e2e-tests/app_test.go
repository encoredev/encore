package tests

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/hashicorp/yamux"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/pubsub"
	"encr.dev/cli/daemon/redis"
	. "encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/secret"
	"encr.dev/internal/builder"
	"encr.dev/internal/builder/builderimpl"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/experiments"
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
	c.Cleanup(func() { ln.Close() })

	app := apps.NewInstance(appRoot, "slug", "")
	mgr := &Manager{}
	rs := NewResourceServices(app, mgr.ClusterMgr /* currently nil */)
	run := &Run{
		ID:              GenID(),
		ListenAddr:      ln.Addr().String(),
		App:             app,
		ResourceServers: rs,
		Mgr:             mgr,
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.Cleanup(cancel)

	parse, build := testBuild(c, appRoot, env)

	jobs := NewAsyncBuildJobs(ctx, app.PlatformOrLocalID(), nil)
	err = run.ResourceServers.StartRequiredServices(jobs, parse.Meta)
	assertNil(err)
	c.Cleanup(rs.StopAll)

	assertNil(jobs.Wait())

	env = append(env, "FOO=bar", "BAR=baz")

	if logger == nil {
		logger = testRunLogger{c}
	}

	expSet, err := experiments.NewSet(nil, env)
	assertNil(err)

	secrets := secret.New()
	secretData, err := secrets.Load(app).Get(ctx, expSet)
	assertNil(err)

	p, err := run.StartProc(&StartProcParams{
		Ctx:            ctx,
		BuildDir:       build.Dir,
		BinPath:        build.Exe,
		Meta:           parse.Meta,
		RuntimePort:    0,
		DBProxyPort:    0,
		Logger:         logger,
		Environ:        env,
		SQLDBCluster:   rs.GetSQLCluster(),
		NSQDaemon:      rs.GetPubSub(),
		Redis:          rs.GetRedis(),
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
	go proxyTcp(ctx, ln, p.Client)

	return &RunAppData{
		Addr:   ln.Addr().String(),
		Run:    run,
		Meta:   parse.Meta,
		NSQ:    rs.GetPubSub(),
		Redis:  rs.GetRedis(),
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

	app := apps.NewInstance(appRoot, "slug", "")
	err := mgr.Test(ctx, TestParams{
		App:          app,
		SQLDBCluster: nil,
		WorkingDir:   ".",
		Args:         []string{"./..."},
		Environ:      environ,
		Stdout:       stdout,
		Stderr:       stderr,
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

func proxyTcp(ctx context.Context, ln net.Listener, client *yamux.Session) {
	for ctx.Err() == nil {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}

			fmt.Printf("unable to accept connection: %+v", err)
			continue
		}

		clientConn, err := client.Open()
		if err != nil {
			fmt.Printf("unable to open connection to running app: %+v", err)
			_ = conn.Close()
			continue
		}

		go io.Copy(conn, clientConn)
		go io.Copy(clientConn, conn)
	}
}

// testBuild is a helper that compiles the app situated at appRoot
// and cleans up the build dir during test cleanup.
func testBuild(t testing.TB, appRoot string, env []string) (*builder.ParseResult, *builder.CompileResult) {
	expSet, err := experiments.NewSet(nil, env)
	if err != nil {
		t.Fatal(err)
	}

	bld := builderimpl.Resolve(expSet)
	ctx := context.Background()

	app := apps.NewInstance(appRoot, t.Name(), "")

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
		os.RemoveAll(build.Dir)
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
