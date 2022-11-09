package run

import (
	"context"
	"fmt"
	"os"

	"encr.dev/cli/internal/appfile"
	"encr.dev/compiler"
	"encr.dev/internal/env"
	"encr.dev/internal/experiments"
	"encr.dev/internal/version"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/vcs"
)

// Check checks the app for errors.
// It reports a buildDir (if available) when codegenDebug is true.
func (mgr *Manager) Check(ctx context.Context, appRoot, relwd string, codegenDebug bool) (buildDir string, err error) {
	vcsRevision := vcs.GetRevision(appRoot)

	exp, err := appfile.Experiments(appRoot)
	if err != nil {
		return "", err
	}
	expSet, err := experiments.NewSet(exp, nil)
	if err != nil {
		return "", err
	}

	// TODO: We should check that all secret keys are defined as well.
	cfg := &compiler.Config{
		Revision:              vcsRevision.Revision,
		UncommittedChanges:    vcsRevision.Uncommitted,
		WorkingDir:            relwd,
		CgoEnabled:            true,
		EncoreCompilerVersion: fmt.Sprintf("EncoreCLI/%s", version.Version),
		EncoreRuntimePath:     env.EncoreRuntimePath(),
		EncoreGoRoot:          env.EncoreGoRoot(),
		KeepOutput:            codegenDebug,
		BuildTags:             []string{"encore_local", "encore_no_gcp", "encore_no_aws", "encore_no_azure"},
		Experiments:           expSet,
		Meta: &cueutil.Meta{
			// Dummy data to satisfy config validation.
			APIBaseURL: "http://localhost:0",
			EnvName:    "encore-check",
			EnvType:    cueutil.EnvType_Development,
			CloudType:  cueutil.CloudType_Local,
		},
	}
	result, err := compiler.Build(appRoot, cfg)
	if result != nil && result.Dir != "" {
		if codegenDebug {
			buildDir = result.Dir
		} else {
			os.RemoveAll(result.Dir)
		}
	}
	return buildDir, err
}
