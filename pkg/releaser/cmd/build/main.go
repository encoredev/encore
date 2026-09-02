// Command build compiles the Encore release artifacts for one or more
// platforms and uploads them to the encore-releases2 bucket under the
// release prefix.
//
// Configuration (environment variables):
//
//	R_VERSION         release version, e.g. "v1.2.3-nightly.20231231"
//	R_RELEASE_PREFIX  object prefix in the bucket; defaults to "encore/<version>"
//	R_ENCORE_REPO     path to the encore repository checkout
//	R_BUILD           space-separated platform specs, e.g.
//	                  "linux/amd64:all,gort,npmpkg,tsparserwasm darwin/arm64:all windows/amd64:all,-jsruntime"
//	R_MACOS_SDK       optional macOS SDK path, only for cross-compiling to darwin from another OS
//	RUNNER_TEMP       scratch directory
//
// Each platform is normally built natively on a runner of that OS (a macOS
// host builds both darwin architectures), with zig providing the C
// toolchain on Linux and Windows; see the workflow.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"encr.dev/internal/version"
	opt "encr.dev/pkg/option"
	. "encr.dev/pkg/releaser/bu"
	"encr.dev/pkg/releaser/config"
	"encr.dev/pkg/releaser/steps/gcsupload"
	"encr.dev/pkg/releaser/steps/gobuild"
	"encr.dev/pkg/releaser/steps/goruntime"
	"encr.dev/pkg/releaser/steps/jsruntime"
	"encr.dev/pkg/releaser/steps/supervisor"
	"encr.dev/pkg/releaser/steps/tsparser"
	"encr.dev/pkg/releaser/steps/tsparserwasm"
)

func join(strs ...string) string {
	return filepath.Join(strs...)
}

var cfg struct {
	config.Common
	EncoreRepo string
	Build      string
	// MacOSSDK is the macOS SDK used when cross-compiling to darwin from
	// another OS (R_MACOS_SDK). Unused on macOS hosts.
	MacOSSDK opt.Option[FSPath]
}

func main() {
	log.Logger = zerolog.New(zerolog.NewConsoleWriter()).With().Caller().Timestamp().Stack().Logger()

	common, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse environment variables")
	}
	cfg.Common = common
	if cfg.EncoreRepo, err = config.Required("R_ENCORE_REPO"); err != nil {
		log.Fatal().Err(err).Msg("failed to parse environment variables")
	}
	if cfg.Build, err = config.Required("R_BUILD"); err != nil {
		log.Fatal().Err(err).Msg("failed to parse environment variables")
	}
	if sdk := os.Getenv("R_MACOS_SDK"); sdk != "" {
		cfg.MacOSSDK = opt.Some(FSPath(sdk))
	}

	log.Info().Msg("starting release")

	repoRoot, err := filepath.Abs(cfg.EncoreRepo)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get encore repo path")
	} else if _, err := os.Stat(join(repoRoot, ".git")); err != nil {
		log.Fatal().Err(err).Msg("expected R_ENCORE_REPO to be the encore repository root")
	}
	cfg.EncoreRepo = repoRoot

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := config.NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create GCS client")
	}

	platforms, err := parseBuildSpec(cfg.Build)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse build spec")
	} else if len(platforms) == 0 {
		log.Fatal().Msg("no platforms specified")
	}
	log.Info().Msg("done parsing build spec")

	var wg sync.WaitGroup
	for _, plat := range platforms {
		wg.Add(1)
		go func(plat PlatformSpec) {
			defer wg.Done()
			spec, err := plat.Build()
			if err != nil {
				log.Fatal().Err(err).Msgf("unable to build")
			} else {
				log.Info().Msg("successfully built")
			}

			log.Info().Msgf("uploading build artifacts to GCS")
			err = gcsupload.Upload(ctx, gcsupload.UploadInput{
				Bucket:  client.Bucket(config.ReleasesBucket),
				Entries: spec.Entries(),
			})
			if err != nil {
				log.Fatal().Err(err).Msg("failed to upload build artifacts to GCS")
			}
			log.Info().Msgf("successfully uploaded build artifacts to GCS")
		}(plat)
	}

	wg.Wait()
}

type ReleaseSpec struct {
	Version            string
	Target             Platform
	NativeModule       opt.Option[*jsruntime.CompileNativeModuleOutput]
	Tsparser           opt.Option[FSPath]
	EncoreCLI          opt.Option[FSPath]
	TSBundler          opt.Option[FSPath]
	GitHook            opt.Option[FSPath]
	Supervisor         opt.Option[*supervisor.CompileOutput]
	NpmPackageTargz    opt.Option[FSPath]
	GoRuntimeTargz     opt.Option[FSPath]
	TSParserWasmPkgDir opt.Option[FSPath]
}

// Entries returns the objects to upload:
//
//	<prefix>/
//	  npmpkg-encore-dev.tar.gz      (npmpkg builds)
//	  encore-go-runtime.tar.gz      (gort builds)
//	  tsparser-wasm/*               (tsparserwasm builds)
//	  <os>-<arch>/
//	    bin/<binaries>
//	    encore-runtime.node(.sha256)
//	    supervisor-encore(.exe)(.sha256)
//
// The finalize-release command assembles the distribution tarballs from
// bin/, encore-runtime.node, the npm package and the Go runtime; the
// supervisor and the checksums are for on-demand download by the CLI.
func (spec ReleaseSpec) Entries() gcsupload.Entries {
	exe := spec.Target.ExeSuffix()

	var bin, version gcsupload.Entries
	if path, ok := spec.Tsparser.Get(); ok {
		bin = append(bin, gcsupload.File{
			Name:   "tsparser-encore" + exe,
			Source: path,
		})
	}

	if path, ok := spec.EncoreCLI.Get(); ok {
		bin = append(bin, gcsupload.File{
			Name:   "encore" + exe,
			Source: path,
		})
	}

	if path, ok := spec.TSBundler.Get(); ok {
		bin = append(bin, gcsupload.File{
			Name:   "tsbundler-encore" + exe,
			Source: path,
		})
	}

	if path, ok := spec.GitHook.Get(); ok {
		bin = append(bin, gcsupload.File{
			Name:   "git-remote-encore" + exe,
			Source: path,
		})
	}

	perArch := gcsupload.Entries{
		gcsupload.Dir{
			Name:    "bin",
			Entries: bin,
		},
	}

	if path, ok := spec.NativeModule.Get(); ok {
		perArch = append(perArch, gcsupload.File{
			Name:   "encore-runtime.node",
			Source: path.NativeModule,
		}, gcsupload.File{
			Name:   "encore-runtime.node.sha256",
			Source: path.Checksum,
		})
	}

	if path, ok := spec.Supervisor.Get(); ok {
		perArch = append(perArch, gcsupload.File{
			Name:   "supervisor-encore" + exe,
			Source: path.SupervisorBinary,
		}, gcsupload.File{
			Name:   "supervisor-encore" + exe + ".sha256",
			Source: path.SupervisorChecksum,
		})
	}

	if path, ok := spec.NpmPackageTargz.Get(); ok {
		version = append(version, gcsupload.File{
			Name:   "npmpkg-encore-dev.tar.gz",
			Source: path,
		})
	}

	if path, ok := spec.GoRuntimeTargz.Get(); ok {
		version = append(version, gcsupload.File{
			Name:   "encore-go-runtime.tar.gz",
			Source: path,
		})
	}

	if pkgDir, ok := spec.TSParserWasmPkgDir.Get(); ok {
		wasmFiles := gcsupload.MustReadDir(pkgDir)
		version = append(version, gcsupload.Dir{
			Name:    "tsparser-wasm",
			Entries: wasmFiles,
		})
	}

	version = append(version, gcsupload.Dir{
		Name:    fmt.Sprintf("%s-%s", spec.Target.OS, spec.Target.Arch),
		Entries: perArch,
	})

	return gcsupload.Entries{gcsupload.Dir{
		Name:    cfg.ReleasePrefix().String(),
		Entries: version,
	}}
}

type PlatformSpec struct {
	Target              Platform
	Workdir             FSPath
	JSRuntime           bool
	TSParser            bool
	EncoreCLI           bool
	GitHook             bool
	TSBundler           bool
	PrepareNpmPackage   bool
	CopyEncoreGoRuntime bool
	Supervisor          bool
	TSParserWasm        bool
}

func parseBuildSpec(val string) ([]PlatformSpec, error) {
	var specs []PlatformSpec
	for _, part := range strings.Split(val, " ") {
		targetStr, val, ok := strings.Cut(part, ":")
		if !ok {
			val = "all"
		}
		target, err := ParsePlatform(targetStr)
		if err != nil {
			return nil, err
		}

		spec, err := parsePlatformSpec(target, val)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

func parsePlatformSpec(target Platform, val string) (PlatformSpec, error) {
	workdir := cfg.TempDir.Join(target.String())
	workdir.MkdirAll()
	spec := PlatformSpec{
		Target:  target,
		Workdir: workdir,
	}

	for _, part := range strings.Split(val, ",") {
		if part == "all" {
			spec.JSRuntime = true
			spec.TSParser = true
			spec.EncoreCLI = true
			spec.TSBundler = true
			spec.GitHook = true
			spec.Supervisor = true
			// Not npmpkg, gort or tsparserwasm here; must be set explicitly.
			continue
		}

		negate := strings.HasPrefix(part, "-")
		if negate {
			part = part[1:]
		}

		var field *bool
		switch part {
		case "jsruntime":
			field = &spec.JSRuntime
		case "tsparser":
			field = &spec.TSParser
		case "cli":
			field = &spec.EncoreCLI
		case "tsbundler":
			field = &spec.TSBundler
		case "supervisor":
			field = &spec.Supervisor
		case "githook":
			field = &spec.GitHook
		case "npmpkg":
			field = &spec.PrepareNpmPackage
		case "gort":
			field = &spec.CopyEncoreGoRuntime
		case "tsparserwasm":
			field = &spec.TSParserWasm
		default:
			return PlatformSpec{}, errors.Newf("unknown platform spec: %s", part)
		}

		if negate {
			*field = false
		} else {
			*field = true
		}
	}
	return spec, nil
}

func (p PlatformSpec) Build() (*ReleaseSpec, error) {
	log.Info().Str("os", p.Target.OS.String()).Str("arch", p.Target.Arch.String()).Msg("building platform")
	p.Workdir.MkdirAll()
	channel := version.ChannelFor(cfg.Version)
	ctx := context.Background()
	cargoCacheBase := FSPath(cfg.EncoreRepo).Join("target", p.Target.OS.String()+"-"+p.Target.Arch.String())
	cargoCacheBase.MkdirAll()

	var napiTypeDefPath opt.Option[FSPath]
	if p.PrepareNpmPackage {
		p := p.Workdir.Join("napi-type-defs.ndjson")
		napiTypeDefPath = opt.Some(p)
	}

	spec := &ReleaseSpec{
		Version:         cfg.Version,
		Target:          p.Target,
		NativeModule:    opt.None[*jsruntime.CompileNativeModuleOutput](),
		Tsparser:        opt.None[FSPath](),
		EncoreCLI:       opt.None[FSPath](),
		TSBundler:       opt.None[FSPath](),
		GitHook:         opt.None[FSPath](),
		NpmPackageTargz: opt.None[FSPath](),
		Supervisor:      opt.None[*supervisor.CompileOutput](),
	}

	host := HostInfo{
		RepoPath: FSPath(cfg.EncoreRepo),
	}

	g, errGroupCtx := errgroup.WithContext(ctx)

	if p.EncoreCLI {
		g.Go(func() error {
			log.Info().Msgf("compiling encore-cli")
			// Windows won't execute a file without the .exe extension, and
			// the smoke test below runs the binary.
			outExePath := p.Workdir.Join("encorecli" + p.Target.ExeSuffix())
			err := gobuild.Exe(errGroupCtx, gobuild.ExeInput{
				Host:            host,
				Target:          p.Target,
				WorkingDir:      host.RepoPath,
				MainPkg:         "./cli/cmd/encore",
				Version:         cfg.Version,
				CrossMacSDKPath: cfg.MacOSSDK,
				OutExePath:      outExePath,
			})
			if err != nil {
				return err
			}
			if err := smokeTestCLI(errGroupCtx, outExePath, p.Target); err != nil {
				return err
			}
			spec.EncoreCLI = opt.Some(outExePath)
			log.Info().Msgf("successfully compiled encore-cli")
			return nil
		})
	}

	if p.TSBundler {
		g.Go(func() error {
			log.Info().Msgf("compiling tsbundler")
			outExePath := p.Workdir.Join("tsbundler-encore")
			err := gobuild.Exe(errGroupCtx, gobuild.ExeInput{
				Host:            host,
				Target:          p.Target,
				WorkingDir:      host.RepoPath,
				MainPkg:         "./cli/cmd/tsbundler-encore",
				Version:         cfg.Version,
				CrossMacSDKPath: cfg.MacOSSDK,
				OutExePath:      outExePath,
			})
			if err != nil {
				return err
			}
			spec.TSBundler = opt.Some(outExePath)
			log.Info().Msgf("successfully compiled tsbundler")
			return nil
		})
	}

	if p.GitHook {
		g.Go(func() error {
			log.Info().Msgf("compiling githook")
			outExePath := p.Workdir.Join("git-remote-encore")
			err := gobuild.Exe(errGroupCtx, gobuild.ExeInput{
				Host:            host,
				Target:          p.Target,
				WorkingDir:      host.RepoPath,
				MainPkg:         "./cli/cmd/git-remote-encore",
				Version:         cfg.Version,
				CrossMacSDKPath: cfg.MacOSSDK,
				OutExePath:      outExePath,
			})
			if err != nil {
				return err
			}
			spec.GitHook = opt.Some(outExePath)
			log.Info().Msgf("successfully compiled githook")
			return nil
		})
	}

	if p.TSParser {
		g.Go(func() error {
			log.Info().Msgf("compiling tsparser")
			tsparserOut, err := tsparser.Compile(errGroupCtx, tsparser.CompileInput{
				Host:            host,
				Target:          p.Target,
				CargoTargetDir:  cargoCacheBase,
				ReleaseBuild:    channel == version.GA,
				ForDistribution: true,
				Version:         cfg.Version,
				CrossMacSDKPath: cfg.MacOSSDK,
			})
			if err != nil {
				return errors.Wrap(err, "failed to compile tsparser")
			}
			log.Info().Msgf("successfully compiled tsparser")
			spec.Tsparser = opt.Some(tsparserOut.TSParserBinary)
			return nil
		})
	}

	if p.Supervisor {
		g.Go(func() error {
			log.Info().Msgf("compiling supervisor")
			supervisorOut, err := supervisor.Compile(errGroupCtx, supervisor.CompileInput{
				Host:            host,
				Target:          p.Target,
				CargoTargetDir:  cargoCacheBase,
				ReleaseBuild:    channel == version.GA,
				ForDistribution: true,
				Version:         cfg.Version,
				CrossMacSDKPath: cfg.MacOSSDK,
			})
			if err != nil {
				return errors.Wrap(err, "failed to compile supervisor")
			}
			log.Info().Msgf("successfully compiled supervisor")
			spec.Supervisor = opt.Some(supervisorOut)
			return nil
		})
	}

	if p.JSRuntime || p.PrepareNpmPackage {
		g.Go(func() error {
			log.Info().Msgf("compiling native module")
			nativeMod, err := jsruntime.CompileNativeModule(errGroupCtx, jsruntime.CompileNativeModuleInput{
				Host:            host,
				Target:          p.Target,
				CargoTargetDir:  cargoCacheBase,
				ReleaseBuild:    channel == version.GA,
				ForDistribution: true,
				NapiTypeDefPath: napiTypeDefPath,
				Version:         cfg.Version,
				CrossMacSDKPath: cfg.MacOSSDK,
			})
			if err != nil {
				return errors.Wrap(err, "failed to compile native module")
			}
			log.Info().Msgf("successfully compiled native module to %s", nativeMod.NativeModule)
			if p.JSRuntime {
				spec.NativeModule = opt.Some(nativeMod)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	if p.PrepareNpmPackage {
		log.Info().Msgf("preparing npm package")
		out, err := jsruntime.PrepareNPMPackage(ctx, jsruntime.PrepareNPMPackageInput{
			Version:         cfg.Version,
			PackagePath:     FSPath(cfg.EncoreRepo).Join("runtimes", "js", "encore.dev"),
			NapiTypeDefPath: napiTypeDefPath.MustGet(),
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to prepare npm package")
		}
		log.Info().Msgf("successfully prepared npm package to %s", out.PackageTargz)
		spec.NpmPackageTargz = opt.Some(out.PackageTargz)
	}

	if p.CopyEncoreGoRuntime {
		// Create a tar.gz of the Go runtime.
		log.Info().Msgf("copying encore-go-runtime")
		src := FSPath(cfg.EncoreRepo).Join("runtimes", "go")
		dst := cfg.TempDir.Join("encore-go-runtime.tar.gz")
		err := goruntime.CreateTargz(ctx, goruntime.CreateTargzInput{
			SrcDir: src,
			Dest:   dst,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create encore-go-runtime tarball")
		}
		spec.GoRuntimeTargz = opt.Some(dst)
	}

	if p.TSParserWasm {
		log.Info().Msgf("compiling tsparser-wasm")
		wasmOut, err := tsparserwasm.Compile(ctx, tsparserwasm.CompileInput{
			Host:           host,
			CargoTargetDir: cargoCacheBase,
			ReleaseBuild:   channel == version.GA,
			Version:        cfg.Version,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to compile tsparser-wasm")
		}
		log.Info().Msgf("successfully compiled tsparser-wasm")
		spec.TSParserWasmPkgDir = opt.Some(wasmOut.PkgDir)
	}

	return spec, nil
}

// smokeTestCLI runs the freshly built CLI's `encore version` and checks that
// it reports the version being built: a start-up check that catches the
// link and runtime-initialization problems a successful compile can't (the
// CLI is a large cgo binary). Only binaries the build host can execute are
// run: same OS, and the same architecture unless the host is an Apple
// Silicon Mac, which runs the darwin/amd64 build through Rosetta (the
// workflow installs it).
func smokeTestCLI(ctx context.Context, exe FSPath, target Platform) error {
	sameOS := target.OS == OS(runtime.GOOS)
	sameArch := target.Arch == Arch(runtime.GOARCH)
	rosetta := target.OS == Darwin && runtime.GOARCH == "arm64" && target.Arch == Amd64
	if !sameOS || !(sameArch || rosetta) {
		log.Warn().Msgf("skipping `encore version` smoke test: this host can't run %s binaries", target)
		return nil
	}

	// `encore version` also asks encore.dev whether an update is available,
	// with a 10s timeout of its own.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	log.Info().Msgf("running `encore version` on the %s build", target)
	out, err := exec.CommandContext(ctx, exe.ToIO(), "version").CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "`encore version` failed for %s:\n%s", target, out)
	}
	want := "encore version " + cfg.Version
	if !strings.Contains(string(out), want) {
		return errors.Newf("unexpected `encore version` output for %s: %q (want %q)", target, out, want)
	}
	log.Info().Msgf("%s", strings.TrimSpace(string(out)))
	return nil
}
