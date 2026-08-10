package run

import (
	"context"
	"fmt"
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

// ExecSpecParams groups the parameters for the ExecSpec method.
type ExecSpecParams struct {
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

	// Environ are the environment variables to set when running the command,
	// in the same format as os.Environ().
	Environ []string

	// TempDir is a path to a temp dir that will be cleaned up by the CLI.
	TempDir string

	OpTracker *optracker.OpTracker
}

// ExecSpecResponse contains the specification for how to run an exec command.
type ExecSpecResponse struct {
	Command string
	Args    []string
	Environ []string
}

// ExecSpec returns the specification for how to run an exec command,
// without actually executing it. This allows the CLI to run the command
// directly with stdin attached for interactive support.
func (mgr *Manager) ExecSpec(ctx context.Context, p ExecSpecParams) (*ExecSpecResponse, error) {
	expSet, err := p.App.Experiments(p.Environ)
	if err != nil {
		return nil, err
	}

	rm := infra.NewResourceManager(p.App, mgr.ClusterMgr, mgr.ObjectsMgr, mgr.PublicBuckets, p.NS, p.Environ, mgr.DBProxyPort, false)

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

	prepareResult, err := bld.Prepare(ctx, builder.PrepareParams{
		Build:      buildInfo,
		App:        p.App,
		WorkingDir: p.WorkingDir,
	})
	if err != nil {
		tracker.Fail(parseOp, errors.New("prepare error"))
		return nil, err
	}
	parse, err := bld.Parse(ctx, builder.ParseParams{
		Build:       buildInfo,
		App:         p.App,
		Experiments: expSet,
		WorkingDir:  p.WorkingDir,
		ParseTests:  false,
		Prepare:     prepareResult,
	})
	if err != nil {
		tracker.Fail(parseOp, errors.New("parse error"))
		return nil, err
	}
	if err := p.App.CacheMetadata(parse.Meta); err != nil {
		return nil, errors.Wrap(err, "cache metadata")
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
		return nil, err
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
		return nil, err
	}

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
		MetaPath:          option.Some(filepath.Join(p.TempDir, "meta.pb")),
		RuntimeConfigPath: option.Some(filepath.Join(p.TempDir, "runtime_config.pb")),
	}
	procConf, err := configGen.AllInOneProc(bld.UseNewRuntimeConfig())
	if err != nil {
		return nil, err
	}
	procEnv, err := configGen.ProcEnvs(procConf, bld.UseNewRuntimeConfig())
	if err != nil {
		return nil, errors.Wrap(err, "compute proc envs")
	}

	defaultEnv := []string{"ENCORE_RUNTIME_LOG=error"}
	env := append(defaultEnv, p.Environ...)
	env = append(env, procConf.ExtraEnv...)
	env = append(env, procEnv...)

	tracker.AllDone()

	return &ExecSpecResponse{
		Command: p.Command,
		Args:    p.ScriptArgs,
		Environ: env,
	}, nil
}
