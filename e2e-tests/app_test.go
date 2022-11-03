package tests

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/yamux"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/pubsub"
	"encr.dev/cli/daemon/redis"
	. "encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/secret"
	"encr.dev/compiler"
	"encr.dev/internal/env"
	"encr.dev/parser"
	"encr.dev/pkg/cueutil"
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

func RunApp(c testing.TB, appRoot string, logger RunLogger) *RunAppData {
	assertNil := func(err error) {
		if err != nil {
			c.Fatal(err)
		}
	}

	ln, err := net.Listen("tcp", "localhost:0")
	assertNil(err)
	c.Cleanup(func() { ln.Close() })

	app := apps.NewInstance("/", "slug", "slug")
	run := &Run{ID: GenID(), ListenAddr: ln.Addr().String(), App: app}
	ctx, cancel := context.WithCancel(context.Background())
	c.Cleanup(cancel)

	nsqd := &pubsub.NSQDaemon{}
	err = nsqd.Start()
	assertNil(err)
	c.Cleanup(nsqd.Stop)

	redisSrv := redis.New()
	err = redisSrv.Start()
	assertNil(err)
	c.Cleanup(redisSrv.Stop)

	build := testBuild(c, appRoot)
	env := []string{"FOO=bar", "BAR=baz"}

	if logger == nil {
		logger = testRunLogger{c}
	}

	p, err := run.StartProc(&StartProcParams{
		Ctx:            ctx,
		BuildDir:       build.Dir,
		BinPath:        build.Exe,
		Meta:           build.Parse.Meta,
		RuntimePort:    0,
		DBProxyPort:    0,
		Logger:         logger,
		Environ:        env,
		NSQDaemon:      nsqd,
		Redis:          redisSrv,
		ServiceConfigs: build.Configs,
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
		Meta:   build.Parse.Meta,
		NSQ:    nsqd,
		Redis:  redisSrv,
		Env:    env,
		Values: make(map[string]any),
	}
}

func RunTests(c testing.TB, appRoot string, stdout, stderr io.Writer, environ []string) error {
	assertNil := func(err error) {
		if err != nil {
			c.Fatal(err)
		}
	}

	mgr := &Manager{
		Secret:     secret.New(),
		ClusterMgr: nil,
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.Cleanup(cancel)

	cfg := &parser.Config{
		AppRoot:                  appRoot,
		AppRevision:              "",
		AppHasUncommittedChanges: false,
		ModulePath:               "test",
		WorkingDir:               ".",
		ParseTests:               true,
	}
	parse, err := parser.Parse(cfg)
	assertNil(err)

	app := apps.NewInstance(appRoot, "slug", "")
	err = mgr.Test(ctx, TestParams{
		App:          app,
		SQLDBCluster: nil,
		WorkingDir:   ".",
		Parse:        parse,
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
func testBuild(t testing.TB, appRoot string) *compiler.Result {
	// Generate use facing code
	err := compiler.GenUserFacing(appRoot)

	// Then compile the app
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	runtimePath := filepath.Join(wd, "../runtime")
	build, err := compiler.Build(appRoot, &compiler.Config{
		EncoreRuntimePath: runtimePath,
		EncoreGoRoot:      env.EncoreGoRoot(),
		Meta: &cueutil.Meta{
			APIBaseURL: "http://what?",
			EnvName:    "end_to_end_test",
			EnvType:    cueutil.EnvType_Development,
			CloudType:  cueutil.CloudType_Local,
		},
		BuildTags: []string{"encore_local", "encore_no_gcp", "encore_no_aws", "encore_no_azure"},
	})
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	t.Cleanup(func() {
		os.RemoveAll(build.Dir)
	})
	return build
}
