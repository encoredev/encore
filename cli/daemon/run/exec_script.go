package run

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/run/infra"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/pkg/vcs"
)

// ExecScriptParams groups the parameters for the ExecScript method.
type ExecScriptParams struct {
	// App is the app to execute the script for.
	App *apps.Instance

	// MainPkg is the package path to the command to execute.
	MainPkg paths.Pkg

	// ScriptArgs are the arguments to pass to the script binary.
	ScriptArgs []string

	// WorkingDir is the working dir to execute the script from.
	// It's relative to the app root.
	WorkingDir string

	// Environ are the environment variables to set when running the tests,
	// in the same format as os.Environ().
	Environ []string

	// Stdout and Stderr are where "go test" output should be written.
	Stdout, Stderr io.Writer

	OpTracker *optracker.OpTracker
}

// ExecScript executes the script.
func (mgr *Manager) ExecScript(ctx context.Context, p ExecScriptParams) (err error) {
	expSet, err := p.App.Experiments(p.Environ)
	if err != nil {
		return err
	}

	rm := infra.NewResourceManager(p.App, mgr.ClusterMgr, p.Environ, false)
	defer rm.StopAll()

	tracker := p.OpTracker
	jobs := optracker.NewAsyncBuildJobs(ctx, p.App.PlatformOrLocalID(), tracker)

	// Parse the app to figure out what infrastructure is needed.
	start := time.Now()
	parseOp := tracker.Add("Building Encore application graph", start)
	topoOp := tracker.Add("Analyzing service topology", start)

	bld := builderimpl.Resolve(expSet)
	vcsRevision := vcs.GetRevision(p.App.Root())
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
		MainPkg:            option.Some(p.MainPkg),
	}

	parse, err := bld.Parse(ctx, builder.ParseParams{
		Build:       buildInfo,
		App:         p.App,
		Experiments: expSet,
		WorkingDir:  p.WorkingDir,
		ParseTests:  false,
	})
	if err != nil {
		tracker.Fail(parseOp, err)
		return err
	}
	tracker.Done(parseOp, 500*time.Millisecond)
	tracker.Done(topoOp, 300*time.Millisecond)

	rm.StartRequiredServices(jobs, parse.Meta)

	var secrets map[string]string
	if usesSecrets(parse.Meta) {
		jobs.Go("Fetching application secrets", true, 150*time.Millisecond, func(ctx context.Context) error {
			data, err := mgr.Secret.Load(p.App).Get(ctx, expSet)
			if err != nil {
				return err
			}
			secrets = data.Values
			return nil
		})
	}

	apiBaseURL := fmt.Sprintf("http://localhost:%d", mgr.RuntimePort)

	var build *builder.CompileResult
	jobs.Go("Compiling application source code", false, 0, func(ctx context.Context) (err error) {
		build, err = bld.Compile(ctx, builder.CompileParams{
			Build:       buildInfo,
			App:         p.App,
			Parse:       parse,
			OpTracker:   tracker,
			Experiments: expSet,
			WorkingDir:  p.WorkingDir,
			CueMeta: &cueutil.Meta{
				APIBaseURL: apiBaseURL,
				EnvName:    "local",
				EnvType:    cueutil.EnvType_Development,
				CloudType:  cueutil.CloudType_Local,
			},
		})
		if err != nil {
			return errors.Wrap(err, "compile error on exec")
		}
		return nil
	})
	defer func() {
		if build != nil {
			os.RemoveAll(build.Dir)
		}
	}()

	if err := jobs.Wait(); err != nil {
		return err
	}

	runtimeCfg, err := mgr.generateConfig(generateConfigParams{
		App:         p.App,
		RM:          rm,
		Meta:        parse.Meta,
		ForTests:    false,
		AuthKey:     genAuthKey(),
		APIBaseURL:  apiBaseURL,
		ConfigAppID: GenID(),
		ConfigEnvID: GenID(),
	})
	if err != nil {
		return err
	}
	runtimeJSON, _ := json.Marshal(runtimeCfg)

	env := append(os.Environ(), p.Environ...)
	env = append(env,
		"ENCORE_RUNTIME_CONFIG="+base64.RawURLEncoding.EncodeToString(runtimeJSON),
		"ENCORE_APP_SECRETS="+encodeSecretsEnv(secrets),
	)
	for serviceName, cfgString := range build.Configs {
		env = append(env, "ENCORE_CFG_"+strings.ToUpper(serviceName)+"="+base64.RawURLEncoding.EncodeToString([]byte(cfgString)))
	}

	tracker.AllDone()

	cmd := exec.CommandContext(ctx, build.Exe, p.ScriptArgs...)
	cmd.Dir = filepath.Join(p.App.Root(), p.WorkingDir)
	cmd.Stdout = p.Stdout
	cmd.Stderr = p.Stderr
	cmd.Env = env
	return cmd.Run()
}
