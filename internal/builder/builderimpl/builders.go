package builderimpl

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
	"encr.dev/pkg/experiments"
	"encr.dev/pkg/vcs"
	"encr.dev/v2/v2builder"
)

func Resolve(expSet *experiments.Set) builder.Impl {
	if experiments.V2.Enabled(expSet) {
		return v2builder.BuilderImpl{}
	} else {
		return Legacy{}
	}
}

type Legacy struct{}

func (Legacy) Parse(p builder.ParseParams) (*builder.ParseResult, error) {
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

func (Legacy) Compile(p builder.CompileParams) (*builder.CompileResult, error) {
	//goland:noinspection HttpUrlsUsage
	cfg := &compiler.Config{
		Revision:              p.Parse.Meta.AppRevision,
		UncommittedChanges:    p.Parse.Meta.UncommittedChanges,
		WorkingDir:            p.WorkingDir,
		EncoreCompilerVersion: fmt.Sprintf("EncoreCLI/%s", version.Version),
		EncoreRuntimePath:     env.EncoreRuntimePath(),
		EncoreGoRoot:          env.EncoreGoRoot(),
		Experiments:           p.Experiments,
		Meta:                  p.CueMeta,
		Parse:                 p.Parse.Data.(*parser.Result),
		OpTracker:             p.OpTracker,

		Debug:               p.Build.Debug,
		KeepOutputOnFailure: p.Build.KeepOutput,
		BuildTags:           p.Build.BuildTags,
		CgoEnabled:          p.Build.CgoEnabled,
		StaticLink:          p.Build.StaticLink,
		GOOS:                p.Build.GOOS,
		GOARCH:              p.Build.GOARCH,
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
