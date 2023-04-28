package run

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"runtime"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/run/infra"
	"encr.dev/cli/daemon/secret"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/vcs"
)

// TestParams groups the parameters for the Test method.
type TestParams struct {
	// App is the app to test.
	App *apps.Instance

	// WorkingDir is the working dir, for formatting
	// error messages with relative paths.
	WorkingDir string

	// Secrets are the secrets to use.
	Secrets *secret.LoadResult

	// Args are the arguments to pass to "go test".
	Args []string

	// Environ are the environment variables to set when running the tests,
	// in the same format as os.Environ().
	Environ []string

	// Stdout and Stderr are where "go test" output should be written.
	Stdout, Stderr io.Writer
}

// Test runs the tests.
func (mgr *Manager) Test(ctx context.Context, params TestParams) (err error) {
	expSet, err := params.App.Experiments(params.Environ)
	if err != nil {
		return err
	}

	secretData, err := params.Secrets.Get(ctx, expSet)
	if err != nil {
		return err
	}
	secrets := secretData.Values

	bld := builderimpl.Resolve(expSet)

	vcsRevision := vcs.GetRevision(params.App.Root())
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
		App:         params.App,
		Experiments: expSet,
		WorkingDir:  params.WorkingDir,
		ParseTests:  true,
	})
	if err != nil {
		return err
	}

	rm := infra.NewResourceManager(params.App, mgr.ClusterMgr, nil, true)
	apiBaseURL := fmt.Sprintf("http://localhost:%d", mgr.RuntimePort)

	jobs := optracker.NewAsyncBuildJobs(ctx, params.App.PlatformOrLocalID(), nil)
	rm.StartRequiredServices(jobs, parse.Meta)

	// Note: jobs.Wait must be called before generateConfig.
	if err := jobs.Wait(); err != nil {
		return err
	}

	runtimeCfg, err := mgr.generateConfig(generateConfigParams{
		App:         params.App,
		RM:          rm,
		Meta:        parse.Meta,
		ForTests:    true,
		AuthKey:     genAuthKey(),
		APIBaseURL:  apiBaseURL,
		ConfigAppID: "test",
		ConfigEnvID: "test",
	})
	if err != nil {
		return err
	}
	runtimeJSON, _ := json.Marshal(runtimeCfg)

	return bld.Test(ctx, builder.TestParams{
		Compile: builder.CompileParams{
			Build:       buildInfo,
			App:         params.App,
			Parse:       parse,
			OpTracker:   nil,
			Experiments: expSet,
			WorkingDir:  params.WorkingDir,
			CueMeta: &cueutil.Meta{
				APIBaseURL: apiBaseURL,
				EnvName:    "local",
				EnvType:    cueutil.EnvType_Test,
				CloudType:  cueutil.CloudType_Local,
			},
		},
		Env: append(params.Environ,
			"ENCORE_RUNTIME_CONFIG="+base64.RawURLEncoding.EncodeToString(runtimeJSON),
			"ENCORE_APP_SECRETS="+encodeSecretsEnv(secrets),
		),
		Args:   params.Args,
		Stdout: params.Stdout,
		Stderr: params.Stderr,
	})
}
