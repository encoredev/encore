package run

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/cockroachdb/errors"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/cli/daemon/run/infra"
	"encr.dev/internal/optracker"
	"encr.dev/internal/version"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	"encr.dev/pkg/promise"
	"encr.dev/pkg/vcs"
)

// ExecCommandParams groups the parameters for the ExecCommand method.
type ExecCommandParams struct {
	// App is the app to execute the script for.
	App *apps.Instance

	// NS is the namespace to use.
	NS *namespace.Namespace

	// Command to execute
	Command string

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

// ExecCommand executes the script.
func (mgr *Manager) ExecCommand(ctx context.Context, p ExecCommandParams) (err error) {
	expSet, err := p.App.Experiments(p.Environ)
	if err != nil {
		return err
	}

	rm := infra.NewResourceManager(p.App, mgr.ClusterMgr, mgr.ObjectsMgr, mgr.PublicBuckets, p.NS, p.Environ, mgr.DBProxyPort, false)
	defer rm.StopAll()

	tracker := p.OpTracker
	jobs := optracker.NewAsyncBuildJobs(ctx, p.App.PlatformOrLocalID(), tracker)

	// Parse the app to figure out what infrastructure is needed.
	start := time.Now()
	parseOp := tracker.Add("Building Encore application graph", start)
	topoOp := tracker.Add("Analyzing service topology", start)

	bld := builderimpl.Resolve(p.App.Lang(), expSet)
	defer fns.CloseIgnore(bld)
	vcsRevision := vcs.GetRevision(p.App.Root())
	buildInfo := builder.BuildInfo{
		BuildTags:          builder.LocalBuildTags,
		CgoEnabled:         true,
		StaticLink:         false,
		DebugMode:          builder.DebugModeDisabled,
		Environ:            p.Environ,
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		KeepOutput:         false,
		Revision:           vcsRevision.Revision,
		UncommittedChanges: vcsRevision.Uncommitted,

		// Use the local JS runtime if this is a development build.
		UseLocalJSRuntime: version.Channel == version.DevBuild,
	}

	parse, err := bld.Parse(ctx, builder.ParseParams{
		Build:       buildInfo,
		App:         p.App,
		Experiments: expSet,
		WorkingDir:  p.WorkingDir,
		ParseTests:  false,
	})
	if err != nil {
		// Don't use the error itself in tracker.Fail, as it will lead to duplicate error output.
		tracker.Fail(parseOp, errors.New("parse error"))
		return err
	}
	if err := p.App.CacheMetadata(parse.Meta); err != nil {
		return errors.Wrap(err, "cache metadata")
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

	configProm := promise.New(func() (*builder.ServiceConfigsResult, error) {
		return bld.ServiceConfigs(ctx, builder.ServiceConfigsParams{
			Parse: parse,
			CueMeta: &cueutil.Meta{
				APIBaseURL: apiBaseURL,
				EnvName:    "local",
				EnvType:    cueutil.EnvType_Development,
				CloudType:  cueutil.CloudType_Local,
			},
		})
	})

	if err := jobs.Wait(); err != nil {
		return err
	}

	gateways := make(map[string]GatewayConfig)
	for _, gw := range parse.Meta.Gateways {
		gateways[gw.EncoreName] = GatewayConfig{
			BaseURL:   apiBaseURL,
			Hostnames: []string{"localhost"},
		}
	}

	cfg, err := configProm.Get(ctx)
	if err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "encore-exec")
	if err != nil {
		return errors.Wrap(err, "couldn't create temp dir")
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	authKey := genAuthKey()
	configGen := &RuntimeConfigGenerator{
		app:               p.App,
		infraManager:      rm,
		md:                parse.Meta,
		AppID:             option.Some(GenID()),
		EnvID:             option.Some(GenID()),
		TraceEndpoint:     option.Some(fmt.Sprintf("http://localhost:%d/trace", mgr.RuntimePort)),
		AuthKey:           authKey,
		Gateways:          gateways,
		DefinedSecrets:    secrets,
		SvcConfigs:        cfg.Configs,
		IncludeMeta:       bld.NeedsMeta(),
		MetaPath:          option.Some(filepath.Join(tempDir, "meta.pb")),
		RuntimeConfigPath: option.Some(filepath.Join(tempDir, "runtime_config.pb")),
	}
	procConf, err := configGen.AllInOneProc(bld.UseNewRuntimeConfig())
	if err != nil {
		return err
	}
	procEnv, err := configGen.ProcEnvs(procConf, bld.UseNewRuntimeConfig())
	if err != nil {
		return errors.Wrap(err, "compute proc envs")
	}

	defaultEnv := []string{"ENCORE_RUNTIME_LOG=error"}
	env := append(defaultEnv, p.Environ...)
	env = append(env, procConf.ExtraEnv...)
	env = append(env, procEnv...)

	tracker.AllDone()

	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(ctx, p.Command, p.ScriptArgs...)
	cmd.Dir = filepath.Join(p.App.Root(), p.WorkingDir)
	cmd.Stdout = p.Stdout
	cmd.Stderr = p.Stderr
	cmd.Env = env
	return cmd.Run()
}
