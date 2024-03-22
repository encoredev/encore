package run

import (
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/cockroachdb/errors"
	"github.com/rs/xid"

	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/cli/daemon/run/infra"
	"encr.dev/cli/daemon/secret"
	"encr.dev/internal/optracker"
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
	bld := builderimpl.Resolve(expSet)
	defer fns.CloseIgnore(bld)

	spec, err := mgr.testSpec(ctx, bld, expSet, params.TestSpecParams)
	if err != nil {
		return err
	}

	workingDir := paths.RootedFSPath(params.App.Root(), params.WorkingDir)
	return bld.RunTests(ctx, builder.RunTestsParams{
		Spec:       spec,
		WorkingDir: workingDir,
		Stdout:     params.Stdout,
		Stderr:     params.Stderr,
	})
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
	bld := builderimpl.Resolve(expSet)
	defer fns.CloseIgnore(bld)

	spec, err := mgr.testSpec(ctx, bld, expSet, &params)
	if err != nil {
		return nil, err
	}
	return &TestSpecResponse{
		Command: spec.Command,
		Args:    spec.Args,
		Environ: spec.Environ,
	}, nil
}

// testSpec returns how to run the tests.
func (mgr *Manager) testSpec(ctx context.Context, bld builder.Impl, expSet *experiments.Set, params *TestSpecParams) (*builder.TestSpecResult, error) {
	secretData, err := params.Secrets.Get(ctx, expSet)
	if err != nil {
		return nil, err
	}
	secrets := secretData.Values

	vcsRevision := vcs.GetRevision(params.App.Root())
	buildInfo := builder.BuildInfo{
		BuildTags:          builder.LocalBuildTags,
		CgoEnabled:         true,
		StaticLink:         false,
		Debug:              false,
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		KeepOutput:         params.CodegenDebug,
		Revision:           vcsRevision.Revision,
		UncommittedChanges: vcsRevision.Uncommitted,
	}

	parse, err := bld.Parse(ctx, builder.ParseParams{
		Build:       buildInfo,
		App:         params.App,
		Experiments: expSet,
		WorkingDir:  params.WorkingDir,
		ParseTests:  true,
	})
	if err != nil {
		return nil, err
	}
	if err := params.App.CacheMetadata(parse.Meta); err != nil {
		return nil, errors.Wrap(err, "cache metadata")
	}

	rm := infra.NewResourceManager(params.App, mgr.ClusterMgr, params.NS, nil, mgr.DBProxyPort, true)

	jobs := optracker.NewAsyncBuildJobs(ctx, params.App.PlatformOrLocalID(), nil)
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

	authKey := genAuthKey()
	configGen := &RuntimeConfigGenerator{
		app:            params.App,
		infraManager:   rm,
		md:             parse.Meta,
		AppID:          option.Some("test"),
		EnvID:          option.Some("test"),
		TraceEndpoint:  option.Some(fmt.Sprintf("http://localhost:%d/trace", mgr.RuntimePort)),
		AuthKey:        authKey,
		Gateways:       gateways,
		DefinedSecrets: secrets,
		SvcConfigs:     cfg.Configs,
		EnvName:        option.Some("test"),
		EnvType:        option.Some(runtimev1.Environment_TYPE_TEST),
		DeployID:       option.Some(fmt.Sprintf("clitest_%s", xid.New().String())),
		IncludeMetaEnv: bld.NeedsMeta(),
	}

	env, err := configGen.ForTests(bld.UseNewRuntimeConfig())
	if err != nil {
		return nil, err
	}
	env = append(env, encodeServiceConfigs(cfg.Configs)...)

	return bld.TestSpec(ctx, builder.TestSpecParams{
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
}
