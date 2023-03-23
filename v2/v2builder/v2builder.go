package v2builder

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"golang.org/x/exp/slices"

	"encr.dev/internal/builder"
	"encr.dev/internal/env"
	"encr.dev/internal/etrace"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/option"
	"encr.dev/pkg/promise"
	"encr.dev/pkg/vfs"
	"encr.dev/v2/app"
	"encr.dev/v2/app/legacymeta"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen"
	"encr.dev/v2/codegen/infragen"
	"encr.dev/v2/compiler/build"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/resource"
)

type BuilderImpl struct{}

func (BuilderImpl) Parse(ctx context.Context, p builder.ParseParams) (*builder.ParseResult, error) {
	return etrace.Sync2(ctx, "", "v2builder.Parse", func(ctx context.Context) (res *builder.ParseResult, err error) {
		defer func() {
			if l, ok := perr.CatchBailout(recover()); ok {
				err = fmt.Errorf("parse error: %s\n", l.FormatErrors())
			}
		}()
		fs := token.NewFileSet()
		errs := perr.NewList(ctx, fs)
		pc := &parsectx.Context{
			Ctx: ctx,
			Log: zerolog.New(zerolog.NewConsoleWriter()),
			Build: parsectx.BuildInfo{
				GOROOT:        paths.RootedFSPath(env.EncoreGoRoot(), "."),
				EncoreRuntime: paths.RootedFSPath(env.EncoreRuntimePath(), "."),

				GOARCH:             p.Build.GOARCH,
				GOOS:               p.Build.GOOS,
				CgoEnabled:         p.Build.CgoEnabled,
				StaticLink:         p.Build.StaticLink,
				Debug:              p.Build.Debug,
				BuildTags:          p.Build.BuildTags,
				Revision:           p.Build.Revision,
				UncommittedChanges: p.Build.UncommittedChanges,
			},
			MainModuleDir: paths.RootedFSPath(p.App.Root(), "."),
			FS:            fs,
			ParseTests:    p.ParseTests,
			Errs:          errs,
		}

		parser := parser.NewParser(pc)
		parserResult := parser.Parse()
		appDesc := app.ValidateAndDescribe(pc, parserResult)
		meta := legacymeta.Compute(pc.Errs, appDesc)
		mainModule := parser.MainModule()

		if pc.Errs.Len() > 0 {
			return nil, errors.New(pc.Errs.FormatErrors())
		}

		return &builder.ParseResult{
			Meta: meta,
			Data: &parseData{
				pc:         pc,
				appDesc:    appDesc,
				mainPkg:    paths.Pkg("./__encore/main"), // TODO
				mainModule: mainModule,
			},
		}, nil
	})
}

type parseData struct {
	pc         *parsectx.Context
	appDesc    *app.Desc
	mainPkg    paths.Pkg
	mainModule *pkginfo.Module
}

func (BuilderImpl) Compile(ctx context.Context, p builder.CompileParams) (*builder.CompileResult, error) {
	return etrace.Sync2(ctx, "", "v2builder.Compile", func(ctx context.Context) (res *builder.CompileResult, err error) {
		defer func() {
			if l, ok := perr.CatchBailout(recover()); ok {
				err = fmt.Errorf("compile error: %s\n", l.FormatErrors())
			}
		}()

		pd := p.Parse.Data.(*parseData)

		gg := codegen.New(pd.pc)
		infragen.Process(gg, pd.appDesc)
		apigen.Process(gg, pd.appDesc, pd.mainModule, option.None[codegen.TestConfig]())

		defer func() {
			if l, ok := perr.CatchBailout(recover()); ok {
				err = fmt.Errorf("compile error: %s\n", l.FormatErrors())
			}
		}()

		configProm := promise.New(func() (configResult, error) {
			return computeConfigs(pd.pc.Errs, pd.appDesc, pd.mainModule, p.CueMeta), nil
		})

		buildResult := build.Build(&build.Config{
			Ctx:        pd.pc,
			Overlays:   gg.Overlays(),
			MainPkg:    pd.mainPkg,
			KeepOutput: p.Build.KeepOutput,
		})

		res = &builder.CompileResult{
			Dir: buildResult.Dir.ToIO(),
			Exe: buildResult.Exe.ToIO(),
		}
		if pd.pc.Errs.Len() > 0 {
			return res, fmt.Errorf("compile error: %s\n", pd.pc.Errs.FormatErrors())
		}

		config, err := configProm.Get(pd.pc.Ctx)
		if err != nil {
			return res, err
		}
		res.Configs = config.configs
		res.ConfigFiles = config.files

		return res, nil
	})
}

func (BuilderImpl) Test(ctx context.Context, p builder.TestParams) error {
	return etrace.Sync1(ctx, "", "v2builder.Test", func(ctx context.Context) (err error) {
		defer func() {
			if l, ok := perr.CatchBailout(recover()); ok {
				err = fmt.Errorf("test failure: %s\n", l.FormatErrors())
			}
		}()

		pd := p.Compile.Parse.Data.(*parseData)

		configProm := promise.New(func() (configResult, error) {
			result := etrace.Async1(ctx, "", "computeConfigs", func(ctx context.Context) configResult {
				return computeConfigs(pd.pc.Errs, pd.appDesc, pd.mainModule, p.Compile.CueMeta)
			})
			return result, nil
		})

		gg := codegen.New(pd.pc)
		etrace.Sync0(ctx, "", "codegen", func(ctx context.Context) {
			testCfg := codegen.TestConfig{}
			for _, pkg := range pd.appDesc.Parse.AppPackages() {
				isTestFile := func(f *pkginfo.File) bool { return f.TestFile }
				hasTestFiles := slices.IndexFunc(pkg.Files, isTestFile) != -1
				if hasTestFiles {
					testCfg.Packages = append(testCfg.Packages, pkg)
				}
			}

			infragen.Process(gg, pd.appDesc)
			apigen.Process(gg, pd.appDesc, pd.mainModule, option.Some(testCfg))
		})

		configs, err := configProm.Get(pd.pc.Ctx)
		if err != nil {
			return err
		}

		envs := make([]string, 0, len(p.Env)+len(configs.configs))
		envs = append(envs, p.Env...)
		for serviceName, cfgString := range configs.configs {
			envs = append(envs, "ENCORE_CFG_"+strings.ToUpper(serviceName)+"="+base64.RawURLEncoding.EncodeToString([]byte(cfgString)))
		}

		build.Test(ctx, &build.TestConfig{
			Config: build.Config{
				Ctx:        pd.pc,
				Overlays:   gg.Overlays(),
				KeepOutput: false,
				Env:        envs,
			},
			Args:       p.Args,
			Stdout:     p.Stdout,
			Stderr:     p.Stderr,
			WorkingDir: paths.RootedFSPath(p.Compile.App.Root(), p.Compile.WorkingDir),
		})

		if pd.pc.Errs.Len() > 0 {
			return fmt.Errorf("compile error: %s\n", pd.pc.Errs.FormatErrors())
		}
		return nil
	})
}

type configResult struct {
	configs map[string]string
	files   fs.FS
}

func computeConfigs(errs *perr.List, desc *app.Desc, mainModule *pkginfo.Module, cueMeta *cueutil.Meta) configResult {
	files := pickupConfigFiles(errs, mainModule)

	// TODO this is technically different from the "app root"
	// but it's close enough for now.
	appRoot := mainModule.RootDir.ToIO()

	// TODO this is a hack until we have proper resource usage tracking
	serviceUsesConfig := make(map[string]bool, len(desc.Services))
	for _, r := range desc.Parse.Resources() {
		if r.Kind() == resource.ConfigLoad {
			if svc, ok := desc.ServiceForPath(r.Package().FSPath); ok {
				serviceUsesConfig[svc.Name] = true
			}
		}
	}

	configs := make(map[string]string, len(desc.Services))
	for _, svc := range desc.Services {
		if !serviceUsesConfig[svc.Name] {
			continue
		}

		rel, err := filepath.Rel(appRoot, svc.FSRoot.ToIO())
		if err != nil {
			errs.Addf(token.NoPos, "unable to compute relative path for service config: %v", err)
			continue
		}
		cfg, err := cueutil.LoadFromFS(files, rel, cueMeta)
		if err != nil {
			errs.Addf(token.NoPos, "unable to load service config: %v", err)
			continue
		}
		cfgData, err := cfg.MarshalJSON()
		if err != nil {
			errs.Addf(token.NoPos, "unable to marshal service config: %v", err)
			continue
		}
		configs[svc.Name] = string(cfgData)
	}
	return configResult{configs, files}
}

func pickupConfigFiles(errs *perr.List, mainModule *pkginfo.Module) fs.FS {
	// Create a virtual filesystem for the config files
	configFiles, err := vfs.FromDir(mainModule.RootDir.ToIO(), func(path string, info fs.DirEntry) bool {
		// any CUE files
		if filepath.Ext(path) == ".cue" {
			return true
		}

		// Pickup any files within a CUE module folder (either at the root of the app or in a subfolder)
		if strings.Contains(path, "/cue.mod/") || strings.HasPrefix(path, "cue.mod/") {
			return true
		}
		return false
	})
	if err != nil {
		errs.AssertStd(fmt.Errorf("unable to package configuration files: %w", err))
	}
	return configFiles
}
