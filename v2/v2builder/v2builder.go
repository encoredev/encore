package v2builder

import (
	"bytes"
	"context"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encr.dev/internal/env"
	"encr.dev/internal/etrace"
	"encr.dev/internal/version"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
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

func New() *BuilderImpl {
	return &BuilderImpl{}
}

type BuilderImpl struct{}

func (i *BuilderImpl) Close() error {
	return nil
}

func (*BuilderImpl) Parse(ctx context.Context, p builder.ParseParams) (*builder.ParseResult, error) {
	return etrace.Sync2(ctx, "", "v2builder.Parse", func(ctx context.Context) (res *builder.ParseResult, err error) {
		defer func() {
			err, _ = perr.CatchBailoutAndPanic(err, recover())
		}()
		fset := token.NewFileSet()
		errs := perr.NewList(ctx, fset)

		runtimesDir := p.Build.EncoreRuntimes.GetOrElseF(func() paths.FS { return paths.FS(env.EncoreRuntimesPath()) })
		pc := &parsectx.Context{
			AppID: option.Some(p.App.PlatformOrLocalID()),
			Ctx:   ctx,
			Log:   p.Build.Logger.GetOrElse(zerolog.New(zerolog.NewConsoleWriter())).Level(zerolog.InfoLevel),
			Build: parsectx.BuildInfo{
				Experiments: p.Experiments,
				// We use GetOrElseF here because GoRoot / Runtime path will panic
				// if they are not set, but we don't want to panic if the option
				// is set.
				GOROOT: p.Build.GoRoot.GetOrElseF(func() paths.FS {
					return paths.RootedFSPath(env.EncoreGoRoot(), ".")
				}),
				EncoreRuntime: runtimesDir.Join("go"),

				GOARCH:             p.Build.GOARCH,
				GOOS:               p.Build.GOOS,
				CgoEnabled:         p.Build.CgoEnabled,
				StaticLink:         p.Build.StaticLink,
				Debug:              p.Build.DebugMode,
				BuildTags:          p.Build.BuildTags,
				Revision:           p.Build.Revision,
				UncommittedChanges: p.Build.UncommittedChanges,
				MainPkg:            p.Build.MainPkg,
			},
			MainModuleDir: paths.RootedFSPath(p.App.Root(), "."),
			FS:            fset,
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

func (*BuilderImpl) Compile(ctx context.Context, p builder.CompileParams) (*builder.CompileResult, error) {
	return etrace.Sync2(ctx, "", "v2builder.Compile", func(ctx context.Context) (res *builder.CompileResult, err error) {
		defer func() {
			err, _ = perr.CatchBailoutAndPanic(err, recover())
		}()

		pd := p.Parse.Data.(*parseData)

		codegenOp := p.OpTracker.Add("Generating boilerplate code", time.Now())

		gg := codegen.New(pd.pc, pd.traceNodes)
		infragen.Process(gg, pd.appDesc)
		staticConfig := apigen.Process(apigen.Params{
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

		compileOp := p.OpTracker.Add("Compiling application source code", time.Now())
		buildResult := build.Build(ctx, &build.Config{
			Ctx:          pd.pc,
			Overlays:     gg.Overlays(),
			MainPkg:      paths.Pkg(p.Build.MainPkg.GetOrElse("./encore_internal/main")),
			KeepOutput:   p.Build.KeepOutput,
			StaticConfig: staticConfig,
			Env:          p.Environ,
		})

		output := &builder.GoBuildOutput{ArtifactDir: buildResult.Dir}
		res = &builder.CompileResult{
			OS:      p.Build.GOOS,
			Arch:    p.Build.GOARCH,
			Outputs: []builder.BuildOutput{output},
		}

		// Set the built binaries according to the multi-proc build setting.
		relExe, err := filepath.Rel(output.ArtifactDir.ToIO(), buildResult.Exe.ToIO())
		if err != nil {
			return nil, errors.Wrap(err, "unable to compute relative path to executable")
		}

		exeFile := builder.ArtifactString("${ARTIFACT_DIR}").Join(relExe)

		cmd := builder.ArtifactStrings{exeFile}

		if p.Build.DebugMode == builder.DebugModeBreak {
			dlvPath, err := exec.LookPath("dlv")
			if err != nil {
				return nil, errors.New("unable to find the dlv debugger. Please install and make sure it's in $PATH.")
			}
			cmd = append(builder.ArtifactStrings{builder.ArtifactString(dlvPath), "--listen=127.0.0.1:2345", "--headless=true", "--api-version=2", "--accept-multiclient", "--wd", builder.ArtifactString(p.App.Root()), "exec"}, cmd...)
		}

		spec := builder.CmdSpec{
			Command:          cmd,
			PrioritizedFiles: builder.ArtifactStrings{exeFile},
		}
		output.Entrypoints = []builder.Entrypoint{{
			Cmd:      spec,
			Services: fns.Map(pd.appDesc.Services, func(svc *app.Service) string { return svc.Name }),
			Gateways: fns.Map(pd.appDesc.Gateways, func(gw *app.Gateway) string { return gw.EncoreName }),
		}}

		// Check if the compile result caused errors and if it did return
		if pd.pc.Errs.Len() > 0 {
			p.OpTracker.Fail(compileOp, pd.pc.Errs.AsError())
			return res, pd.pc.Errs.AsError()
		}
		p.OpTracker.Done(compileOp, 450*time.Millisecond)

		// Then check if the config generation caused an error
		if pd.pc.Errs.Len() > 0 {
			return res, pd.pc.Errs.AsError()
		}

		return res, nil
	})
}

func (*BuilderImpl) ServiceConfigs(ctx context.Context, p builder.ServiceConfigsParams) (res *builder.ServiceConfigsResult, err error) {
	defer func() {
		if l, ok := perr.CatchBailout(recover()); ok && l.Len() > 0 {
			err = l.AsError()
		}
	}()

	pd := p.Parse.Data.(*parseData)
	cfg := computeConfigs(pd.pc.Errs, pd.appDesc, pd.mainModule, p.CueMeta)
	if err := pd.pc.Errs.AsError(); err != nil {
		return nil, err
	}
	return &builder.ServiceConfigsResult{
		Configs:     cfg.configs,
		ConfigFiles: cfg.files,
	}, nil
}

func (*BuilderImpl) UseNewRuntimeConfig() bool {
	return false
}

func (*BuilderImpl) NeedsMeta() bool {
	return false
}

func (i *BuilderImpl) RunTests(ctx context.Context, p builder.RunTestsParams) error {
	return etrace.Sync1(ctx, "", "v2builder.Test", func(ctx context.Context) (err error) {
		defer func() {
			err, _ = perr.CatchBailoutAndPanic(err, recover())
		}()

		data, ok := p.Spec.BuilderData.(*testBuilderData)
		if !ok {
			return errors.Newf("invalid builder data type %T", p.Spec.BuilderData)
		}

		build.RunTests(ctx, data.spec, &build.RunTestsConfig{
			Stdout:     p.Stdout,
			Stderr:     p.Stderr,
			WorkingDir: p.WorkingDir,
		})

		if data.pc.Errs.Len() > 0 {
			return data.pc.Errs.AsError()
		}

		return nil
	})
}

type testBuilderData struct {
	spec *build.TestSpec
	pc   *parsectx.Context
}

func (i *BuilderImpl) TestSpec(ctx context.Context, p builder.TestSpecParams) (*builder.TestSpecResult, error) {
	return etrace.Sync2(ctx, "", "v2builder.TestSpec", func(ctx context.Context) (res *builder.TestSpecResult, err error) {
		spec, err := i.generateTestSpec(ctx, p)
		if err != nil {
			return nil, err
		}

		pd := p.Compile.Parse.Data.(*parseData)
		data := &testBuilderData{
			spec: spec,
			pc:   pd.pc,
		}

		return &builder.TestSpecResult{
			Command:     spec.Command,
			Args:        spec.Args,
			Environ:     spec.Environ,
			BuilderData: data,
		}, nil
	})
}

func (i *BuilderImpl) generateTestSpec(ctx context.Context, p builder.TestSpecParams) (*build.TestSpec, error) {
	return etrace.Sync2(ctx, "", "v2builder.generateTestSpec", func(ctx context.Context) (res *build.TestSpec, err error) {
		defer func() {
			err, _ = perr.CatchBailoutAndPanic(err, recover())
		}()

		pd := p.Compile.Parse.Data.(*parseData)

		gg := codegen.New(pd.pc, pd.traceNodes)
		staticConfig := etrace.Sync1(ctx, "", "codegen", func(ctx context.Context) *config.Static {
			testCfg := codegen.TestConfig{}
			for _, pkg := range pd.appDesc.Parse.AppPackages() {
				isTestFile := func(f *pkginfo.File) bool { return f.TestFile }
				hasTestFiles := slices.IndexFunc(pkg.Files, isTestFile) != -1
				if hasTestFiles {
					testCfg.Packages = append(testCfg.Packages, pkg)
				}
			}
			testCfg.EnvsToEmbed = i.testEnvVarsToEmbed(p.Args, p.Env)

			infragen.Process(gg, pd.appDesc)
			return apigen.Process(apigen.Params{
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

		spec := build.GenerateTestSpec(ctx, &build.GenerateTestSpecConfig{
			Config: build.Config{
				Ctx:          pd.pc,
				Overlays:     gg.Overlays(),
				KeepOutput:   p.Compile.Build.KeepOutput,
				Env:          p.Env,
				StaticConfig: staticConfig,
			},
			Args: p.Args,
		})

		if pd.pc.Errs.Len() > 0 {
			return nil, pd.pc.Errs.AsError()
		}
		return spec, nil
	})
}

// testEnvVars takes a list of env vars and filters them down to the ones
// that should be embedded within the test binary.
func (i *BuilderImpl) testEnvVarsToEmbed(args, envs []string) map[string]string {
	if !slices.Contains(args, "-c") {
		return nil
	}

	toEmbed := make(map[string]string)
	for _, e := range envs {
		if strings.HasPrefix(e, "ENCORE_") {
			if key, value, ok := strings.Cut(e, "="); ok {
				toEmbed[key] = value
			}
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
	for _, b := range desc.Parse.AllBinds() {
		r := desc.Parse.ResourceForBind(b)
		if r.Kind() == resource.ConfigLoad {
			if svc, ok := desc.ServiceForPath(b.Package().FSPath); ok {
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
		// Convert the path since io/fs operates on forward slashes.
		rel = filepath.ToSlash(rel)
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
	inCueModFolder := func(path string, info fs.DirEntry) bool {
		// If it's not a directory, get the parent directory
		if !info.IsDir() {
			path = filepath.Dir(path)
		}

		for range 30 {
			base := filepath.Base(path)
			if strings.ToLower(base) == "cue.mod" {
				return true
			}

			parent := filepath.Dir(path)
			if parent == path {
				break
			}
			path = parent
		}
		return false
	}

	// Create a virtual filesystem for the config files
	configFiles, err := vfs.FromDir(mainModule.RootDir.ToIO(), func(path string, info fs.DirEntry) bool {
		return filepath.Ext(path) == ".cue" || inCueModFolder(path, info)
	})

	if err != nil {
		errs.AssertStd(fmt.Errorf("unable to package configuration files: %w", err))
	}
	return configFiles
}

func (i *BuilderImpl) GenUserFacing(ctx context.Context, p builder.GenUserFacingParams) error {
	return etrace.Sync1(ctx, "", "v2builder.GenUserFacing", func(ctx context.Context) (err error) {
		defer func() {
			err, _ = perr.CatchBailoutAndPanic(err, recover())
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

				buf.Reset()
				if f, ok := userfacinggen.Gen(gg, svc, svcStruct).Get(); ok {
					if err := f.Render(&buf); err != nil {
						errs.Addf(token.NoPos, "unable to render userfacing go code: %v", err)
						continue
					}
				}

				i.writeOrDeleteFile(errs, buf.Bytes(), svc.FSRoot.Join("encore.gen.go"))
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
func (i *BuilderImpl) writeOrDeleteFile(errs *perr.List, data []byte, dst paths.FS) {
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
