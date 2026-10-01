package run

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/run/infra"
	"encr.dev/pkg/builder"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

func TestTestResourceLifetime(t *testing.T) {
	commandErr := errors.New("test command failed")
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "success"},
		{name: "test failure", err: commandErr},
		{name: "launch failure", err: &os.PathError{Op: "fork/exec", Path: "test-command", Err: os.ErrNotExist}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr, params, rm, bld := testResourceFixture(t)
			started := make(chan struct{})
			finish := make(chan struct{})
			done := make(chan error, 1)
			bld.run = func(context.Context, builder.RunTestsParams) error {
				close(started)
				<-finish
				return tc.err
			}
			go func() {
				done <- mgr.testWithResources(context.Background(), bld, nil, params, rm)
			}()
			select {
			case <-started:
			case err := <-done:
				t.Fatalf("preparation failed before command started: %v", err)
			}
			// Always release and join the command, even if the assertion fails.
			func() {
				defer close(finish)
				if srv := rm.GetRedis(); srv == nil || srv.Miniredis().Server() == nil {
					t.Error("resources stopped while the test command was still running")
				}
			}()
			if err := <-done; !errors.Is(err, tc.err) {
				t.Errorf("Test error = %v, want %v", err, tc.err)
			}
			assertTestRedisStopped(t, rm)
		})
	}
}

func TestTestResourceCancellation(t *testing.T) {
	mgr, params, rm, bld := testResourceFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	canceled := make(chan struct{})
	finish := make(chan struct{})
	done := make(chan error, 1)
	bld.run = func(ctx context.Context, _ builder.RunTestsParams) error {
		close(started)
		<-ctx.Done()
		close(canceled)
		// Model a command which has received cancellation but is still exiting.
		<-finish
		return ctx.Err()
	}
	go func() {
		done <- mgr.testWithResources(ctx, bld, nil, params, rm)
	}()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("preparation failed: %v", err)
	}
	cancel()
	<-canceled
	if srv := rm.GetRedis(); srv == nil || srv.Miniredis().Server() == nil {
		t.Error("resources stopped on cancellation before the command exited")
	}
	close(finish)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("Test error = %v, want cancellation", err)
	}
	assertTestRedisStopped(t, rm)
}

func TestTestResourceRollback(t *testing.T) {
	prepareErr := errors.New("preparation failed")
	for _, stage := range []string{"prepare", "parse", "startup", "service config", "runtime config", "test spec"} {
		t.Run(stage, func(t *testing.T) {
			mgr, params, rm, bld := testResourceFixture(t)
			switch stage {
			case "prepare":
				bld.prepareErr = prepareErr
			case "parse":
				bld.parseErr = prepareErr
			case "startup":
				// SQL startup fails without a cluster manager, concurrently with
				// Redis startup. Rollback must wait for Redis to be registered.
				bld.md.SqlDatabases = []*meta.SQLDatabase{{Name: "db"}}
			case "service config":
				bld.serviceConfigs = func() error { return prepareErr }
			case "runtime config":
				// A file cannot contain the generated runtime-config file.
				params.TempDir = filepath.Join(t.TempDir(), "not-a-directory")
				if err := os.WriteFile(params.TempDir, nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "test spec":
				bld.spec = func() error { return prepareErr }
			}
			bld.run = func(context.Context, builder.RunTestsParams) error {
				t.Error("RunTests called after failed preparation")
				return nil
			}
			if err := mgr.testWithResources(context.Background(), bld, nil, params, rm); err == nil {
				t.Fatal("expected preparation error")
			}
			if stage == "prepare" || stage == "parse" {
				if rm.GetRedis() != nil {
					t.Fatal("infrastructure started before parsing completed")
				}
			} else {
				assertTestRedisStopped(t, rm)
			}
		})
	}
}

func TestTestResourcePanic(t *testing.T) {
	for _, stage := range []string{"service config", "test spec", "run"} {
		t.Run(stage, func(t *testing.T) {
			mgr, params, rm, bld := testResourceFixture(t)
			boom := errors.New("test panic")
			panicFn := func() error { panic(boom) }
			switch stage {
			case "service config":
				bld.serviceConfigs = panicFn
			case "test spec":
				bld.spec = panicFn
			case "run":
				bld.run = func(context.Context, builder.RunTestsParams) error { return panicFn() }
			}
			func() {
				defer func() {
					if got := recover(); got != boom {
						t.Errorf("panic = %v, want %v", got, boom)
					}
				}()
				_ = mgr.testWithResources(context.Background(), bld, nil, params, rm)
			}()
			assertTestRedisStopped(t, rm)
		})
	}
}

func TestTestSpecRetainsResources(t *testing.T) {
	mgr, params, rm, bld := testResourceFixture(t)
	if _, err := mgr.testSpec(context.Background(), bld, nil, params.TestSpecParams, rm); err != nil {
		t.Fatal(err)
	}
	if srv := rm.GetRedis(); srv == nil || srv.Miniredis().Server() == nil {
		t.Fatal("successful spec must remain usable after preparation returns")
	}
	// The harness owns this exported spec and explicitly ends its lifetime.
	rm.StopAll()
	assertTestRedisStopped(t, rm)
}

func TestTestResourcesIndependent(t *testing.T) {
	mgr1, params1, rm1, bld1 := testResourceFixture(t)
	mgr2, params2, rm2, bld2 := testResourceFixture(t)
	if _, err := mgr1.testSpec(context.Background(), bld1, nil, params1.TestSpecParams, rm1); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr2.testSpec(context.Background(), bld2, nil, params2.TestSpecParams, rm2); err != nil {
		t.Fatal(err)
	}
	rm1.StopAll()
	assertTestRedisStopped(t, rm1)
	if rm2.GetRedis().Miniredis().Server() == nil {
		t.Fatal("ending one spec stopped another spec's resources")
	}
	rm2.StopAll()
	assertTestRedisStopped(t, rm2)
}

func TestTestTypeScriptFlagsDoNotExportGoBinary(t *testing.T) {
	mgr, params, rm, bld := testResourceFixture(t)
	if err := os.WriteFile(filepath.Join(params.App.Root(), "encore.app"), []byte(`{"id":"","lang":"typescript"}`), 0600); err != nil {
		t.Fatal(err)
	}
	params.Args = []string{"-c"} // A test-runner option, not Go's compile flag.
	if err := mgr.testWithResources(context.Background(), bld, nil, params, rm); err != nil {
		t.Fatal(err)
	}
	assertTestRedisStopped(t, rm)
}

func TestTestBinaryPreparationFailure(t *testing.T) {
	mgr, params, rm, bld := testResourceFixture(t)
	params.Args = []string{"-c"}
	prepareErr := errors.New("could not generate test spec")
	bld.spec = func() error { return prepareErr }
	if err := mgr.testWithResources(context.Background(), bld, nil, params, rm); !errors.Is(err, prepareErr) {
		t.Errorf("Test error = %v, want %v", err, prepareErr)
	}
	assertTestRedisStopped(t, rm)
}

func TestTestBinaryResourceLifetime(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		environ []string
		fail    bool
	}{
		{name: "compile", args: []string{"-c"}},
		{name: "compile equals", args: []string{"--c=true"}},
		{name: "args as regex value", args: []string{"-run", "-args", "-c"}},
		{name: "output", args: []string{"-o", "tests.bin"}},
		{name: "GOFLAGS", environ: []string{"GOFLAGS='-o=tests with spaces.bin'"}},
		{name: "compile error retains existing lifetime", args: []string{"-c"}, fail: true},
		{name: "tests fail after binary output", args: []string{"-o=tests.bin"}, fail: true},
		{name: "profile binary", args: []string{"-cpuprofile=cpu.prof"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr, params, rm, bld := testResourceFixture(t)
			params.Args, params.Environ = tc.args, tc.environ
			commandErr := errors.New("compile failed")
			bld.run = func(context.Context, builder.RunTestsParams) error {
				if tc.fail {
					return commandErr
				}
				return nil
			}
			err := mgr.testWithResources(context.Background(), bld, nil, params, rm)
			if tc.fail {
				if !errors.Is(err, commandErr) {
					t.Errorf("Test error = %v, want %v", err, commandErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if rm.GetRedis().Miniredis().Server() == nil {
				t.Fatal("binary-export command lost its infrastructure")
			}
			// End the exported binary's lifetime explicitly in this test.
			rm.StopAll()
			assertTestRedisStopped(t, rm)
		})
	}
}

func TestTestExportsBinary(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		environ []string
		want    bool
	}{
		{name: "ordinary", args: []string{"./...", "-count=1"}},
		{name: "compile", args: []string{"./svc", "-c"}, want: true},
		{name: "double dash", args: []string{"--c"}, want: true},
		{name: "equals true", args: []string{"-c=true"}, want: true},
		{name: "equals one", args: []string{"-c=1"}, want: true},
		{name: "equals false", args: []string{"--c=false"}},
		{name: "equals zero", args: []string{"-c=0"}},
		{name: "output", args: []string{"-o", "tests.bin"}, want: true},
		{name: "output equals", args: []string{"--o=tests.bin"}, want: true},
		{name: "profile", args: []string{"-cpuprofile=cpu.prof"}, want: true},
		{name: "test profile alias", args: []string{"-test.memprofile", "mem.prof"}, want: true},
		{name: "empty output", args: []string{"-o="}},
		{name: "conservative terminator", args: []string{"--", "-c"}, want: true},
		{name: "conservative test args", args: []string{"-args", "-o", "test-data"}, want: true},
		{name: "conservative double dash args", args: []string{"--args", "-c"}, want: true},
		{name: "args is a flag value", args: []string{"-run", "-args", "-c"}, want: true},
		{name: "terminator is a flag value", args: []string{"-run", "--", "-c"}, want: true},
		{name: "env compile", environ: []string{"GOFLAGS=-c"}, want: true},
		{name: "env ordinary", environ: []string{"GOFLAGS=-race -count=1"}},
		{name: "env quoted", environ: []string{"GOFLAGS=\"-o=test binary\""}, want: true},
		{name: "env terminator inside a value", environ: []string{"GOFLAGS=\"-ldflags=-X p.value=--\" -c"}, want: true},
		{name: "env last wins", environ: []string{"GOFLAGS=-c", "GOFLAGS="}},
		{name: "conservative override", args: []string{"-c=false"}, environ: []string{"GOFLAGS=-c"}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := testExportsBinary(tc.args, tc.environ); got != tc.want {
				t.Errorf("testExportsBinary(%q, %q) = %v, want %v", tc.args, tc.environ, got, tc.want)
			}
		})
	}
}

// A real Redis server gives these ownership tests a narrow, observable resource
// without an Encore compiler, child process, Docker, or a shared daemon. Its
// Stop also panics on a second call, detecting duplicate cleanup ownership.
func testResourceFixture(t *testing.T) (*Manager, TestParams, *infra.ResourceManager, *testLifecycleBuilder) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "encore.app"), []byte(`{"id":""}`), 0600); err != nil {
		t.Fatal(err)
	}
	mgr := &Manager{}
	params := TestParams{TestSpecParams: &TestSpecParams{
		App: apps.NewInstance(root, "resource-lifetime-test", ""), TempDir: t.TempDir(),
	}}
	rm := mgr.newTestResources(params.TestSpecParams)
	t.Cleanup(func() {
		// Fallback cleanup only for a still-live server, so a failed assertion
		// cannot leave a resource running or double-stop a released server.
		if srv := rm.GetRedis(); srv != nil && srv.Miniredis().Server() != nil {
			rm.StopAll()
		}
	})
	bld := &testLifecycleBuilder{md: &meta.Data{CacheClusters: []*meta.CacheCluster{{Name: "cache"}}}}
	return mgr, params, rm, bld
}

func assertTestRedisStopped(t *testing.T, rm *infra.ResourceManager) {
	t.Helper()
	srv := rm.GetRedis()
	if srv == nil {
		t.Fatal("test did not provision Redis")
	}
	if srv.Miniredis().Server() != nil {
		t.Fatal("Redis was not stopped before the operation returned")
	}
	ln, err := net.Listen("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("Redis listener was not released: %v", err)
	}
	_ = ln.Close()
}

type testLifecycleBuilder struct {
	builder.Impl   // Any unexpected builder operation fails the test.
	md             *meta.Data
	prepareErr     error
	parseErr       error
	serviceConfigs func() error
	spec           func() error
	run            func(context.Context, builder.RunTestsParams) error
}

func (b *testLifecycleBuilder) Prepare(context.Context, builder.PrepareParams) (*builder.PrepareResult, error) {
	return &builder.PrepareResult{}, b.prepareErr
}

func (b *testLifecycleBuilder) Parse(context.Context, builder.ParseParams) (*builder.ParseResult, error) {
	return &builder.ParseResult{Meta: b.md}, b.parseErr
}

func (b *testLifecycleBuilder) ServiceConfigs(context.Context, builder.ServiceConfigsParams) (*builder.ServiceConfigsResult, error) {
	if b.serviceConfigs != nil {
		if err := b.serviceConfigs(); err != nil {
			return nil, err
		}
	}
	return &builder.ServiceConfigsResult{}, nil
}

func (b *testLifecycleBuilder) TestSpec(context.Context, builder.TestSpecParams) (*builder.TestSpecResult, error) {
	if b.spec != nil {
		if err := b.spec(); err != nil {
			return nil, err
		}
	}
	return &builder.TestSpecResult{Command: "test-command"}, nil
}

func (b *testLifecycleBuilder) RunTests(ctx context.Context, p builder.RunTestsParams) error {
	if b.run != nil {
		return b.run(ctx, p)
	}
	return nil
}

func (*testLifecycleBuilder) UseNewRuntimeConfig() bool { return true }
func (*testLifecycleBuilder) NeedsMeta() bool           { return false }
