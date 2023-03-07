package run

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"

	"encr.dev/compiler"
	"encr.dev/internal/builder"
	"encr.dev/internal/env"
	"encr.dev/internal/version"
	"encr.dev/parser"
	"encr.dev/pkg/vcs"
)

type legacyBuilderImpl struct{}

func (legacyBuilderImpl) Parse(p builder.ParseParams) (*builder.ParseResult, error) {
	modPath := filepath.Join(p.App.Root(), "go.mod")
	modData, err := os.ReadFile(modPath)
	if err != nil {
		return nil, err
	}
	mod, err := modfile.Parse(modPath, modData, nil)
	if err != nil {
		return nil, err
	}

	vcsRevision := vcs.GetRevision(p.App.Root())

	cfg := &parser.Config{
		AppRoot:                  p.App.Root(),
		Experiments:              p.Experiments,
		AppRevision:              vcsRevision.Revision,
		AppHasUncommittedChanges: vcsRevision.Uncommitted,
		ModulePath:               mod.Module.Mod.Path,
		WorkingDir:               p.WorkingDir,
		ParseTests:               p.ParseTests,
		ScriptMainPkg:            p.ScriptMainPkg,
	}

	res, err := parser.Parse(cfg)
	if err != nil {
		return nil, err
	}
	return &builder.ParseResult{
		Meta: res.Meta,
		Data: res,
	}, nil
}

func (legacyBuilderImpl) Compile(p builder.CompileParams) (*builder.CompileResult, error) {
	//goland:noinspection HttpUrlsUsage
	cfg := &compiler.Config{
		Revision:              p.Parse.Meta.AppRevision,
		UncommittedChanges:    p.Parse.Meta.UncommittedChanges,
		WorkingDir:            p.WorkingDir,
		CgoEnabled:            true,
		EncoreCompilerVersion: fmt.Sprintf("EncoreCLI/%s", version.Version),
		EncoreRuntimePath:     env.EncoreRuntimePath(),
		EncoreGoRoot:          env.EncoreGoRoot(),
		Experiments:           p.Experiments,
		Meta:                  p.CueMeta,
		Parse:                 p.Parse.Data.(*parser.Result),
		BuildTags:             []string{"encore_local", "encore_no_gcp", "encore_no_aws", "encore_no_azure"},
		OpTracker:             p.OpTracker,
	}

	build, err := compiler.Build(p.App.Root(), cfg)
	if err != nil {
		return nil, err
	}
	return &builder.CompileResult{
		Dir:     build.Dir,
		Exe:     build.Exe,
		Configs: build.Configs,
	}, nil
}
