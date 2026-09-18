package run

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/cockroachdb/errors"
	"github.com/rs/xid"

	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/cli/daemon/run/infra"
	"encr.dev/cli/daemon/secret"
	"encr.dev/internal/optracker"
	"encr.dev/internal/version"
	"encr.dev/pkg/appfile"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/pkg/vcs"
	runtimev1 "encr.dev/proto/encore/runtime/v1"
)

// TestParams groups the parameters for the Test method.
type TestParams struct {
	*TestSpecParams

	// Stdout and Stderr are where "go test" output should be written.
	Stdout, Stderr io.Writer
}

// Test runs the tests.
func (mgr *Manager) Test(ctx context.Context, params TestParams) (err error) {
	expSet, err := params.App.Experiments(params.Environ)
	if err != nil {
		return err
	}
	bld := builderimpl.Resolve(params.App.Lang(), expSet)
	defer fns.CloseIgnore(bld)

	return mgr.testWithResources(ctx, bld, expSet, params, mgr.newTestResources(params.TestSpecParams))
}

// testWithResources owns the infrastructure for a synchronous test command.
// Binary-export commands retain their infrastructure for later execution,
// just like TestSpec; neither currently has an explicit session-release API.
func (mgr *Manager) testWithResources(ctx context.Context, bld builder.Impl, expSet *experiments.Set, params TestParams, rm *infra.ResourceManager) error {
	spec, err := mgr.testSpec(ctx, bld, expSet, params.TestSpecParams, rm)
	if err != nil {
		return err
	}
	// A nonzero command exit does not prove there is no exported binary: with
	// -o, for example, compilation may succeed before the tests fail.
	retainResources := params.App.Lang() == appfile.LangGo &&
		!experiments.TypeScript.Enabled(expSet) && testExportsBinary(params.Args, params.Environ)
	defer func() {
		if !retainResources {
			rm.StopAll()
		}
	}()

	workingDir := paths.RootedFSPath(params.App.Root(), params.WorkingDir)
	return bld.RunTests(ctx, builder.RunTestsParams{
		Spec:       spec,
		WorkingDir: workingDir,
		Stdout:     params.Stdout,
		Stderr:     params.Stderr,
	})
}

// testExportsBinary conservatively recognizes flags that can leave a test
// binary for later use. This is an ownership check, not a replacement for Go's
// flag parser: ambiguous arguments should retain resources rather than break a
// later execution. Export-mode completion, including errors after preparation,
// needs a separate artifact/session lifetime before it can safely be cleaned up.
func testExportsBinary(args, environ []string) bool {
	exports := func(args []string) bool {
		for _, arg := range args {
			// Do not stop at -args or --: either may be another flag's value
			// (for example, go test -run -args -c still exports a binary).
			// Forwarded flags with these names conservatively retain resources.
			if !strings.HasPrefix(arg, "-") {
				continue
			}
			name, value, hasValue := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "=")
			switch name {
			case "c":
				if !hasValue {
					return true
				}
				if compile, err := strconv.ParseBool(value); err != nil || compile {
					return true
				}
			case "o", "cpuprofile", "memprofile", "blockprofile", "mutexprofile",
				"test.cpuprofile", "test.memprofile", "test.blockprofile", "test.mutexprofile":
				// Go keeps the test binary for these profiling flags as well.
				if !hasValue || value != "" {
					return true
				}
			}
		}
		return false
	}
	if exports(args) {
		return true
	}
	// GOFLAGS also configures go test. Splitting quoted contents conservatively
	// may retain resources for a flag value, but must not miss a binary export.
	for i := len(environ) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(environ[i], "GOFLAGS="); ok {
			return exports(strings.FieldsFunc(value, func(r rune) bool {
				return unicode.IsSpace(r) || r == '\'' || r == '"'
			}))
		}
	}
	return false
}

// TestSpecParams are the parameters for computing a test spec.
type TestSpecParams struct {
	// App is the app to test.
	App *apps.Instance

	// NS is the namespace to use.
	NS *namespace.Namespace

	// Secrets are the secrets to use.
	Secrets *secret.LoadResult

	// Args are the arguments to pass to the test command.
	Args []string

	// WorkingDir is the working dir, for formatting
	// error messages with relative paths.
	WorkingDir string

	// Environ are the environment variables to set when running the tests,
	// in the same format as os.Environ().
	Environ []string

	// CodegenDebug, if true, specifies to keep the output
	// around for codegen debugging purposes.
	CodegenDebug bool

	// TempDir is a path to a temp dir that will be clean up by the test runner.
	TempDir string
}

type TestSpecResponse struct {
	Command string
	Args    []string
	Environ []string
}

// TestSpec returns how to run the tests.
func (mgr *Manager) TestSpec(ctx context.Context, params TestSpecParams) (*TestSpecResponse, error) {
	expSet, err := params.App.Experiments(params.Environ)
	if err != nil {
		return nil, err
	}
	bld := builderimpl.Resolve(params.App.Lang(), expSet)
	defer fns.CloseIgnore(bld)

	// The client executes this spec after the RPC returns. Only unsuccessful
	// preparation can be cleaned up here; successful specs need a future session
	// completion protocol before their resources can be released.
	spec, err := mgr.testSpec(ctx, bld, expSet, &params, mgr.newTestResources(&params))
	if err != nil {
		return nil, err
	}
	return &TestSpecResponse{
		Command: spec.Command,
		Args:    spec.Args,
		Environ: spec.Environ,
	}, nil
}

func (mgr *Manager) newTestResources(params *TestSpecParams) *infra.ResourceManager {
	return infra.NewResourceManager(params.App, mgr.ClusterMgr, mgr.ObjectsMgr, mgr.PublicBuckets, params.NS, nil, mgr.DBProxyPort, true)
}

// testSpec owns rm until it successfully returns a spec. On success the caller
// takes ownership; on error or panic preparation releases the infrastructure.
func (mgr *Manager) testSpec(ctx context.Context, bld builder.Impl, expSet *experiments.Set, params *TestSpecParams, rm *infra.ResourceManager) (*builder.TestSpecResult, error) {
	prepared := false
	defer func() {
		if !prepared {
			rm.StopAll()
		}
	}()

	var secrets map[string]string
	if params.Secrets != nil {
		secretData, err := params.Secrets.Get(ctx, expSet)
		if err != nil {
			return nil, err
		}
		secrets = secretData.Values
		// remove db override secrets for tests
		for k, _ := range secrets {
			if strings.HasPrefix(k, "sqldb::") {
				delete(secrets, k)
			}
		}
	}

	vcsRevision := vcs.GetRevision(params.App.Root())
	buildInfo := builder.BuildInfo{
		BuildTags:          builder.LocalBuildTags,
		CgoEnabled:         true,
		StaticLink:         false,
		DebugMode:          builder.DebugModeDisabled,
		Environ:            params.Environ,
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		KeepOutput:         params.CodegenDebug,
		Revision:           vcsRevision.Revision,
		UncommittedChanges: vcsRevision.Uncommitted,

		// Use the local JS runtime if this is a development build.
		UseLocalJSRuntime: version.Channel == version.DevBuild,
	}

	prepareResult, err := bld.Prepare(ctx, builder.PrepareParams{
		Build:      buildInfo,
		App:        params.App,
		WorkingDir: params.WorkingDir,
	})
	if err != nil {
		return nil, err
	}
	parse, err := bld.Parse(ctx, builder.ParseParams{
		Build:       buildInfo,
		App:         params.App,
		Experiments: expSet,
		WorkingDir:  params.WorkingDir,
		ParseTests:  true,
		Prepare:     prepareResult,
	})
	if err != nil {
		return nil, err
	}
	if err := params.App.CacheMetadata(parse.Meta); err != nil {
		return nil, errors.Wrap(err, "cache metadata")
	}

	jobs := optracker.NewAsyncBuildJobs(ctx, params.App.PlatformOrLocalID(), nil)
	// A panic must not let rollback race a service that is still starting.
	// This defer runs before the resource cleanup above.
	defer jobs.Wait()
	rm.StartRequiredServices(jobs, parse.Meta)

	// Note: jobs.Wait must be called before generateConfig.
	if err := jobs.Wait(); err != nil {
		return nil, err
	}

	gateways := make(map[string]GatewayConfig)
	gatewayBaseURL := fmt.Sprintf("http://localhost:%d", mgr.RuntimePort)
	for _, gw := range parse.Meta.Gateways {
		gateways[gw.EncoreName] = GatewayConfig{
			BaseURL:   gatewayBaseURL,
			Hostnames: []string{"localhost"},
		}
	}

	cfg, err := bld.ServiceConfigs(ctx, builder.ServiceConfigsParams{
		Parse: parse,
		CueMeta: &cueutil.Meta{
			APIBaseURL: gatewayBaseURL,
			EnvName:    "local",
			EnvType:    cueutil.EnvType_Test,
			CloudType:  cueutil.CloudType_Local,
		},
	})
	if err != nil {
		return nil, err
	}

	var runtimeConfigPath option.Option[string]
	var metaPath option.Option[string]

	if params.TempDir != "" {
		if bld.UseNewRuntimeConfig() {
			runtimeConfigPath = option.Some(filepath.Join(params.TempDir, "runtime_config.pb"))
		} else {
			runtimeConfigPath = option.Some(filepath.Join(params.TempDir, "runtime_config.json"))
		}

		if bld.NeedsMeta() {
			metaPath = option.Some(filepath.Join(params.TempDir, "meta.pb"))
		}
	}

	authKey := genAuthKey()
	configGen := &RuntimeConfigGenerator{
		app:               params.App,
		infraManager:      rm,
		md:                parse.Meta,
		AppID:             option.Some(params.App.PlatformOrLocalID()),
		EnvID:             option.Some("test"),
		TraceEndpoint:     option.Some(fmt.Sprintf("http://localhost:%d/trace", mgr.RuntimePort)),
		AuthKey:           authKey,
		Gateways:          gateways,
		DefinedSecrets:    secrets,
		SvcConfigs:        cfg.Configs,
		EnvName:           option.Some("test"),
		EnvType:           option.Some(runtimev1.Environment_TYPE_TEST),
		DeployID:          option.Some(fmt.Sprintf("clitest_%s", xid.New().String())),
		IncludeMeta:       bld.NeedsMeta(),
		MetaPath:          metaPath,
		RuntimeConfigPath: runtimeConfigPath,
	}

	env, err := configGen.ForTests(bld.UseNewRuntimeConfig())
	if err != nil {
		return nil, err
	}
	env = append(env, encodeServiceConfigs(cfg.Configs)...)

	spec, err := bld.TestSpec(ctx, builder.TestSpecParams{
		Compile: builder.CompileParams{
			Build:       buildInfo,
			App:         params.App,
			Parse:       parse,
			OpTracker:   nil,
			Experiments: expSet,
			WorkingDir:  params.WorkingDir,
		},
		Env:  append(params.Environ, env...),
		Args: params.Args,
	})
	prepared = err == nil
	return spec, err
}
