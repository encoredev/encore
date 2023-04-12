package builderimpl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"golang.org/x/mod/modfile"

	"encr.dev/compiler"
	"encr.dev/internal/env"
	"encr.dev/internal/version"
	"encr.dev/parser"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/experiments"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/v2builder"
)

func Resolve(expSet *experiments.Set) builder.Impl {
	return v2builder.BuilderImpl{}
}

type Legacy struct{}

func (Legacy) Parse(ctx context.Context, p builder.ParseParams) (*builder.ParseResult, error) {
	modPath := filepath.Join(p.App.Root(), "go.mod")
	modData, err := os.ReadFile(modPath)
	if err != nil {
		return nil, err
	}
	mod, err := modfile.Parse(modPath, modData, nil)
	if err != nil {
		return nil, err
	}

	cfg := &parser.Config{
		AppRoot:                  p.App.Root(),
		Experiments:              p.Experiments,
		AppRevision:              p.Build.Revision,
		AppHasUncommittedChanges: p.Build.UncommittedChanges,
		ModulePath:               mod.Module.Mod.Path,
		WorkingDir:               p.WorkingDir,
		ParseTests:               p.ParseTests,
		ScriptMainPkg:            p.Build.MainPkg.GetOrElse("").String(),
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

func (l Legacy) Compile(ctx context.Context, p builder.CompileParams) (*builder.CompileResult, error) {
	cfg := l.compilerConfig(p)

	var (
		build *compiler.Result
		err   error
	)
	if cfg.ExecScript != nil {
		build, err = compiler.ExecScript(p.App.Root(), cfg)
	} else {
		build, err = compiler.Build(p.App.Root(), cfg)
	}
	if err != nil {
		return nil, err
	}

	return &builder.CompileResult{
		Dir:         build.Dir,
		Exe:         build.Exe,
		Configs:     build.Configs,
		ConfigFiles: build.ConfigFiles,
	}, nil
}

func (Legacy) compilerConfig(p builder.CompileParams) *compiler.Config {
	cfg := &compiler.Config{
		Revision:              p.Parse.Meta.AppRevision,
		UncommittedChanges:    p.Parse.Meta.UncommittedChanges,
		WorkingDir:            p.WorkingDir,
		EncoreCompilerVersion: p.EncoreVersion.GetOrElse(fmt.Sprintf("EncoreCLI/%s", version.Version)),
		// We use GetOrElseF here because GoRoot / Runtime path will panic
		// if they are not set, but we don't want to panic if the option
		// is set.
		EncoreRuntimePath: option.Map(p.Build.EncoreRuntime, paths.FS.ToIO).GetOrElseF(func() string { return env.EncoreRuntimePath() }),
		EncoreGoRoot:      option.Map(p.Build.GoRoot, paths.FS.ToIO).GetOrElseF(func() string { return env.EncoreGoRoot() }),
		Experiments:       p.Experiments,
		Meta:              p.CueMeta,
		Parse:             p.Parse.Data.(*parser.Result),
		OpTracker:         p.OpTracker,
		Log:               p.Build.Logger.GetOrElse(zerolog.New(zerolog.NewConsoleWriter())),

		Debug:               p.Build.Debug,
		KeepOutputOnFailure: p.Build.KeepOutput,
		BuildTags:           p.Build.BuildTags,
		CgoEnabled:          p.Build.CgoEnabled,
		StaticLink:          p.Build.StaticLink,
		GOOS:                p.Build.GOOS,
		GOARCH:              p.Build.GOARCH,
	}

	if relPath, ok := p.Build.MainPkg.Get(); ok {
		cfg.ExecScript = &compiler.ExecScriptConfig{
			ScriptMainPkg: relPath.String(),
		}
	}

	return cfg
}

func (l Legacy) Test(ctx context.Context, p builder.TestParams) error {
	cfg := l.compilerConfig(p.Compile)
	cfg.Test = &compiler.TestConfig{
		Env:    p.Env,
		Args:   p.Args,
		Stdout: p.Stdout,
		Stderr: p.Stderr,
	}
	return compiler.Test(ctx, p.Compile.App.Root(), cfg)
}

func (l Legacy) GenUserFacing(ctx context.Context, p builder.GenUserFacingParams) error {
	return compiler.GenUserFacing(p.App.Root(), p.Parse.Data.(*parser.Result))
}
