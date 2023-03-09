package legacybuild

import (
	"context"
	"errors"
	"fmt"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rs/zerolog"

	"encr.dev/internal/builder"
	"encr.dev/internal/env"
	"encr.dev/pkg/cueutil"
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
	"encr.dev/v2/parser/infra/resource"
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
}

type parseData struct {
	pc         *parsectx.Context
	appDesc    *app.Desc
	mainPkg    paths.Pkg
	mainModule *pkginfo.Module
}

func (BuilderImpl) Compile(p builder.CompileParams) (res *builder.CompileResult, err error) {
	pd := p.Parse.Data.(*parseData)

	gg := codegen.New(pd.pc)
	infragen.Process(gg, pd.appDesc)
	apigen.Process(gg, pd.appDesc, pd.mainModule)

	defer func() {
		if l, ok := perr.CatchBailout(recover()); ok {
			res = nil
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
		KeepOutput: false,
	})
	if pd.pc.Errs.Len() > 0 {
		return nil, fmt.Errorf("compile error: %s\n", pd.pc.Errs.FormatErrors())
	}

	config, err := configProm.Get(pd.pc.Ctx)
	if err != nil {
		return nil, err
	}

	return &builder.CompileResult{
		Dir:         buildResult.Dir.ToIO(),
		Exe:         buildResult.Exe.ToIO(),
		Configs:     config.configs,
		ConfigFiles: config.files,
	}, nil
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
	for _, r := range desc.Infra.Resources() {
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
