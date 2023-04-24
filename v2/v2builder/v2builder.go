package v2builder

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/exp/slices"

	"encr.dev/internal/env"
	"encr.dev/internal/etrace"
	"encr.dev/internal/version"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/pkg/promise"
	"encr.dev/pkg/vfs"
	"encr.dev/v2/app"
	"encr.dev/v2/app/legacymeta"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen"
	"encr.dev/v2/codegen/apigen/userfacinggen"
	"encr.dev/v2/codegen/cuegen"
	"encr.dev/v2/codegen/infragen"
	"encr.dev/v2/compiler/build"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/resource"
)

type BuilderImpl struct{}

func (BuilderImpl) Parse(ctx context.Context, p builder.ParseParams) (*builder.ParseResult, error) {
	return etrace.Sync2(ctx, "", "v2builder.Parse", func(ctx context.Context) (res *builder.ParseResult, err error) {
		defer func() {
			if l, ok := perr.CatchBailout(recover()); ok {
				err = l.AsError()
			}
		}()
		fs := token.NewFileSet()
		errs := perr.NewList(ctx, fs)
		pc := &parsectx.Context{
			Ctx: ctx,
			Log: p.Build.Logger.GetOrElse(zerolog.New(zerolog.NewConsoleWriter())).Level(zerolog.InfoLevel),
			Build: parsectx.BuildInfo{
				Experiments: p.Experiments,
				// We use GetOrElseF here because GoRoot / Runtime path will panic
				// if they are not set, but we don't want to panic if the option
				// is set.
				GOROOT: p.Build.GoRoot.GetOrElseF(func() paths.FS {
					return paths.RootedFSPath(env.EncoreGoRoot(), ".")
				}),
				EncoreRuntime: p.Build.EncoreRuntime.GetOrElseF(func() paths.FS {
					return paths.RootedFSPath(env.EncoreRuntimePath(), ".")
				}),

				GOARCH:             p.Build.GOARCH,
				GOOS:               p.Build.GOOS,
				CgoEnabled:         p.Build.CgoEnabled,
				StaticLink:         p.Build.StaticLink,
				Debug:              p.Build.Debug,
				BuildTags:          p.Build.BuildTags,
				Revision:           p.Build.Revision,
				UncommittedChanges: p.Build.UncommittedChanges,
				MainPkg:            p.Build.MainPkg,
			},
			MainModuleDir: paths.RootedFSPath(p.App.Root(), "."),
			FS:            fs,
			ParseTests:    p.ParseTests,
			Errs:          errs,
		}

		parser := parser.NewParser(pc)
		parserResult := parser.Parse()
		appDesc := app.ValidateAndDescribe(pc, parserResult)
		meta, traceNodes := legacymeta.Compute(pc.Errs, appDesc)
		mainModule := parser.MainModule()
		runtimeModule := parser.RuntimeModule()

		if pc.Errs.Len() > 0 {
			return nil, pc.Errs.AsError()
		}

		return &builder.ParseResult{
			Meta: meta,
			Data: &parseData{
				pc:            pc,
				appDesc:       appDesc,
				mainModule:    mainModule,
				runtimeModule: runtimeModule,
				traceNodes:    traceNodes,
			},
		}, nil
	})
}

type parseData struct {
	pc            *parsectx.Context
	appDesc       *app.Desc
	mainModule    *pkginfo.Module
	runtimeModule *pkginfo.Module
	traceNodes    *legacymeta.TraceNodes
}

func (BuilderImpl) Compile(ctx context.Context, p builder.CompileParams) (*builder.CompileResult, error) {
	return etrace.Sync2(ctx, "", "v2builder.Compile", func(ctx context.Context) (res *builder.CompileResult, err error) {
		defer func() {
			if l, ok := perr.CatchBailout(recover()); ok && l.Len() > 0 {
				err = l.AsError()
			}
		}()

		pd := p.Parse.Data.(*parseData)

		codegenOp := p.OpTracker.Add("Generating boilerplate code", time.Now())

		gg := codegen.New(pd.pc, pd.traceNodes)
		infragen.Process(gg, pd.appDesc)
		apigen.Process(apigen.Params{
			Gen:               gg,
			Desc:              pd.appDesc,
			MainModule:        pd.mainModule,
			RuntimeModule:     pd.runtimeModule,
			CompilerVersion:   p.EncoreVersion.GetOrElse(fmt.Sprintf("EncoreCLI/%s", version.Version)),
			AppRevision:       p.Build.Revision,
			AppUncommitted:    p.Build.UncommittedChanges,
			ExecScriptMainPkg: p.Build.MainPkg,
		})

		if pd.pc.Errs.Len() > 0 {
			p.OpTracker.Fail(codegenOp, pd.pc.Errs.AsError())
			return res, pd.pc.Errs.AsError()
		}
		p.OpTracker.Done(codegenOp, 450*time.Millisecond)

		configProm := promise.New(func() (res configResult, err error) {
			defer func() {
				if l, ok := perr.CatchBailout(recover()); ok && l.Len() > 0 {
					err = l.AsError()
				}
			}()

			return computeConfigs(pd.pc.Errs, pd.appDesc, pd.mainModule, p.CueMeta), nil
		})

		compileOp := p.OpTracker.Add("Compiling application source code", time.Now())
		buildResult := build.Build(ctx, &build.Config{
			Ctx:        pd.pc,
			Overlays:   gg.Overlays(),
			MainPkg:    paths.Pkg(p.Build.MainPkg.GetOrElse("./__encore/main")),
			KeepOutput: p.Build.KeepOutput,
		})

		res = &builder.CompileResult{
			Dir: buildResult.Dir.ToIO(),
			Exe: buildResult.Exe.ToIO(),
		}
		// Check if the compile result caused errors and if it did return
		if pd.pc.Errs.Len() > 0 {
			p.OpTracker.Fail(compileOp, pd.pc.Errs.AsError())
			return res, pd.pc.Errs.AsError()
		}
		p.OpTracker.Done(compileOp, 450*time.Millisecond)

		config, err := configProm.Get(pd.pc.Ctx)
		if err != nil {
			return res, err
		}
		res.Configs = config.configs
		res.ConfigFiles = config.files

		// Then check if the config generation caused an error
		if pd.pc.Errs.Len() > 0 {
			return res, pd.pc.Errs.AsError()
		}

		return res, nil
	})
}

func (i BuilderImpl) Test(ctx context.Context, p builder.TestParams) error {
	return etrace.Sync1(ctx, "", "v2builder.Test", func(ctx context.Context) (err error) {
		defer func() {
			if l, ok := perr.CatchBailout(recover()); ok {
				err = l.AsError()
			}
		}()

		pd := p.Compile.Parse.Data.(*parseData)

		configs := computeConfigs(pd.pc.Errs, pd.appDesc, pd.mainModule, p.Compile.CueMeta)

		envs := make([]string, 0, len(p.Env)+len(configs.configs))
		envs = append(envs, p.Env...)
		for serviceName, cfgString := range configs.configs {
			envs = append(envs, "ENCORE_CFG_"+strings.ToUpper(serviceName)+"="+base64.RawURLEncoding.EncodeToString([]byte(cfgString)))
		}

		gg := codegen.New(pd.pc, pd.traceNodes)
		etrace.Sync0(ctx, "", "codegen", func(ctx context.Context) {
			testCfg := codegen.TestConfig{}
			for _, pkg := range pd.appDesc.Parse.AppPackages() {
				isTestFile := func(f *pkginfo.File) bool { return f.TestFile }
				hasTestFiles := slices.IndexFunc(pkg.Files, isTestFile) != -1
				if hasTestFiles {
					testCfg.Packages = append(testCfg.Packages, pkg)
				}
			}
			testCfg.EnvsToEmbed = i.testEnvVarsToEmbed(p, envs)

			infragen.Process(gg, pd.appDesc)
			apigen.Process(apigen.Params{
				Gen:             gg,
				Desc:            pd.appDesc,
				MainModule:      pd.mainModule,
				RuntimeModule:   pd.runtimeModule,
				CompilerVersion: p.Compile.EncoreVersion.GetOrElse(fmt.Sprintf("EncoreCLI/%s", version.Version)),
				AppRevision:     p.Compile.Build.Revision,
				AppUncommitted:  p.Compile.Build.UncommittedChanges,
				Test:            option.Some(testCfg),
			})
		})

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
			return pd.pc.Errs.AsError()
		}
		return nil
	})
}

// testEnvVars takes a list of env vars and filters them down to the ones
// that should be embedded within the test binary.
func (i BuilderImpl) testEnvVarsToEmbed(p builder.TestParams, envs []string) []string {
	if !slices.Contains(p.Args, "-c") {
		return nil
	}

	var toEmbed []string
	for _, e := range envs {
		if strings.HasPrefix(e, "ENCORE_") {
			toEmbed = append(toEmbed, e)
		}
	}
	return toEmbed
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
	serviceUsesConfig := make(map[string]resource.Resource, len(desc.Services))
	for _, r := range desc.Parse.Resources() {
		if r.Kind() == resource.ConfigLoad {
			if svc, ok := desc.ServiceForPath(r.Package().FSPath); ok {
				serviceUsesConfig[svc.Name] = r
			}
		}
	}

	configs := make(map[string]string, len(desc.Services))
	for _, svc := range desc.Services {
		resourceNode, ok := serviceUsesConfig[svc.Name]
		if !ok {
			continue
		}

		rel, err := filepath.Rel(appRoot, svc.FSRoot.ToIO())
		if err != nil {
			errs.AddStdNode(err, resourceNode)
			continue
		}
		cfg, err := cueutil.LoadFromFS(files, rel, cueMeta)
		if err != nil {
			errs.AddStdNode(err, resourceNode)
			continue
		}
		cfgData, err := cfg.MarshalJSON()
		if err != nil {
			errs.AddStdNode(err, resourceNode)
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

func (i BuilderImpl) GenUserFacing(ctx context.Context, p builder.GenUserFacingParams) error {
	return etrace.Sync1(ctx, "", "v2builder.GenUserFacing", func(ctx context.Context) (err error) {
		defer func() {
			if l, ok := perr.CatchBailout(recover()); ok {
				err = l.AsError()
			}
		}()

		pd := p.Parse.Data.(*parseData)
		errs := pd.pc.Errs
		gg := codegen.New(pd.pc, pd.traceNodes)
		cueGen := cuegen.NewGenerator(pd.appDesc)

		var buf bytes.Buffer
		for _, svc := range pd.appDesc.Services {
			// Generate the user-facing Go code.
			{
				// Service structs are not needed if there is no implementation to be generated
				svcStruct := option.None[*codegen.VarDecl]()

				if f, ok := userfacinggen.Gen(gg, svc, svcStruct).Get(); ok {
					buf.Reset()
					if err := f.Render(&buf); err != nil {
						errs.Addf(token.NoPos, "unable to render userfacing go code: %v", err)
						continue
					}
					dst := svc.FSRoot.Join("encore.gen.go")
					i.writeOrDeleteFile(errs, buf.Bytes(), dst)
				}
			}

			// Generate the user-facing CUE code.
			{
				data, err := cueGen.UserFacing(svc)
				if err != nil {
					errs.AddStd(err)
					continue
				}
				dst := svc.FSRoot.Join("encore.gen.cue")
				i.writeOrDeleteFile(errs, data, dst)
			}
		}

		if errs.Len() > 0 {
			return errs.AsError()
		}

		return nil
	})
}

// writeOrDeleteFile writes the given data to dst. If data is empty, it will
// instead delete the file at dst.
func (i BuilderImpl) writeOrDeleteFile(errs *perr.List, data []byte, dst paths.FS) {
	if len(data) == 0 {
		// No need for any generated code. Try to remove the existing file
		// if it's there as it's no longer needed.
		_ = os.Remove(dst.ToIO())
	} else {
		if err := os.WriteFile(dst.ToIO(), data, 0644); err != nil {
			errs.AddStd(err)
		}
	}
}
