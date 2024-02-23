package run

import (
	"context"
	"runtime"

	"github.com/cockroachdb/errors"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/vcs"
)

type CheckParams struct {
	// App is the app to start.
	App *apps.Instance

	// WorkingDir is the working dir, for formatting
	// error messages with relative paths.
	WorkingDir string

	// CodegenDebug, if true, specifies to keep the output
	// around for codegen debugging purposes.
	CodegenDebug bool

	// Environ are the environment variables to set,
	// in the same format as os.Environ().
	Environ []string

	// Tests specifies whether to parse and codegen for tests as well.
	Tests bool
}

// Check checks the app for errors.
// It reports a buildDir (if available) when codegenDebug is true.
func (mgr *Manager) Check(ctx context.Context, p CheckParams) (buildDir string, err error) {
	expSet, err := p.App.Experiments(p.Environ)
	if err != nil {
		return "", err
	}

	// TODO: We should check that all secret keys are defined as well.

	vcsRevision := vcs.GetRevision(p.App.Root())
	buildInfo := builder.BuildInfo{
		BuildTags:          builder.LocalBuildTags,
		CgoEnabled:         true,
		StaticLink:         false,
		Debug:              false,
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		KeepOutput:         p.CodegenDebug,
		Revision:           vcsRevision.Revision,
		UncommittedChanges: vcsRevision.Uncommitted,
	}

	bld := builderimpl.Resolve(expSet)
	parse, err := bld.Parse(ctx, builder.ParseParams{
		Build:       buildInfo,
		App:         p.App,
		Experiments: expSet,
		WorkingDir:  p.WorkingDir,
		ParseTests:  p.Tests,
	})
	if err != nil {
		return "", err
	}
	if err := p.App.CacheMetadata(parse.Meta); err != nil {
		return "", errors.Wrap(err, "cache metadata")
	}

	// Validate the service configs.
	_, err = bld.ServiceConfigs(ctx, builder.ServiceConfigsParams{
		Parse: parse,
		CueMeta: &cueutil.Meta{
			// Dummy data to satisfy config validation.
			APIBaseURL: "http://localhost:0",
			EnvName:    "encore-check",
			EnvType:    cueutil.EnvType_Development,
			CloudType:  cueutil.CloudType_Local,
		},
	})
	if err != nil {
		return "", err
	}

	result, err := bld.Compile(ctx, builder.CompileParams{
		Build:       buildInfo,
		App:         p.App,
		Parse:       parse,
		OpTracker:   nil, // TODO
		Experiments: expSet,
		WorkingDir:  p.WorkingDir,
	})

	if result != nil && len(result.Outputs) > 0 {
		buildDir = result.Outputs[0].GetArtifactDir().ToIO()
	}
	return buildDir, err
}
