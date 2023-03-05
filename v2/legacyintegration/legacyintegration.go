package legacyintegration

import (
	"context"
	"errors"
	"fmt"
	"go/token"
	"runtime"

	"github.com/rs/zerolog"

	"encr.dev/internal/builder"
	"encr.dev/internal/env"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen"
	"encr.dev/v2/codegen/infragen"
	"encr.dev/v2/compiler/build"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/legacyintegration/legacymeta"
	"encr.dev/v2/parser"
)

type BuilderImpl struct{}

func (BuilderImpl) Parse(p builder.ParseParams) (*builder.ParseResult, error) {
	ctx := context.Background()
	fs := token.NewFileSet()
	errs := perr.NewList(ctx, fs)
	pc := &parsectx.Context{
		Ctx: ctx,
		Log: zerolog.New(zerolog.NewConsoleWriter()),
		Build: parsectx.BuildInfo{
			GOARCH:        runtime.GOARCH,
			GOOS:          runtime.GOOS,
			GOROOT:        paths.RootedFSPath(env.EncoreGoRoot(), "."),
			EncoreRuntime: paths.RootedFSPath(env.EncoreRuntimePath(), "."),

			// TODO(andre) make these configurable?
			CgoEnabled: false,
			StaticLink: false,
			Debug:      false,

			// TODO(andre) Do we need all this still?
			BuildTags: []string{"encore_local", "encore_no_gcp", "encore_no_aws", "encore_no_azure"},
		},
		MainModuleDir: paths.RootedFSPath(p.App.Root(), p.WorkingDir),
		FS:            fs,
		ParseTests:    p.ParseTests,
		Errs:          errs,
	}

	parser := parser.NewParser(pc)
	parserResult := parser.Parse()
	meta := legacymeta.Gen(pc.Errs, parserResult)

	if pc.Errs.Len() > 0 {
		return nil, errors.New(pc.Errs.FormatErrors())
	}

	return &builder.ParseResult{
		Meta: meta,
		Data: &parseData{
			pc:      pc,
			res:     parserResult,
			mainPkg: paths.Pkg("./cmd/main"), // TODO
		},
	}, nil
}

type parseData struct {
	pc      *parsectx.Context
	res     parser.Result
	mainPkg paths.Pkg
}

func (BuilderImpl) Compile(p builder.CompileParams) (res *builder.CompileResult, err error) {
	pd := p.Parse.Data.(*parseData)

	gg := codegen.New(pd.pc)
	infragen.Process(gg, pd.res.Infra)

	if pd.res.Framework.IsPresent() {
		apigen.Process(gg, pd.res.Framework.MustGet())
	}

	defer func() {
		if l, ok := perr.CatchBailout(recover()); ok {
			res = nil
			err = fmt.Errorf("compile error: %s\n", l.FormatErrors())
		}
	}()

	buildResult := build.Build(&build.Config{
		Ctx:        pd.pc,
		Overlays:   gg.Overlays(),
		MainPkg:    pd.mainPkg,
		KeepOutput: false,
	})
	if pd.pc.Errs.Len() > 0 {
		return nil, fmt.Errorf("compile error: %s\n", pd.pc.Errs.FormatErrors())
	}
	return &builder.CompileResult{
		Dir:     buildResult.Dir.ToIO(),
		Exe:     buildResult.Exe.ToIO(),
		Configs: nil, // TODO
	}, nil
}
