package jsruntime

import (
	"context"
	"fmt"
	"runtime"

	"encr.dev/pkg/releaser/steps/rustbuild"
	"github.com/cockroachdb/errors"

	"encr.dev/pkg/option"
	. "encr.dev/pkg/releaser/bu"
)

type CompileNativeModuleInput struct {
	Host   HostInfo
	Target Platform

	// The cargo 'target' directory to use for compiled outputs.
	CargoTargetDir FSPath

	// The path to the macOS SDK. Only needed when cross-compiling to macOS
	// from another OS; native macOS builds use the Xcode toolchain.
	CrossMacSDKPath option.Option[FSPath]

	// Optional file path to write NAPI type definitions to.
	// Must be set if we're generating NAPI type definitions in this build.
	NapiTypeDefPath option.Option[FSPath]

	// Version string to embed in the native module.
	Version string

	// True if it should be built with `--release`.
	ReleaseBuild bool

	// True if the output is for distribution, as opposed to
	// only running on the host machine (or running tests).
	//
	// Ensures the output is compatible with common glibc versions,
	// for example.
	ForDistribution bool
}

type CompileNativeModuleOutput struct {
	// Path to the built native module.
	// Will be within the provided OutputDir.
	NativeModule FSPath
	Checksum     FSPath
}

// CompileNativeModule compiles the native module for the JS runtime.
func CompileNativeModule(ctx context.Context, in CompileNativeModuleInput) (*CompileNativeModuleOutput, error) {
	artifactName := func() string {
		switch in.Target.OS {
		case Darwin:
			return "libencore_js_runtime.dylib"
		case Linux:
			return "libencore_js_runtime.so"
		case Windows:
			return "encore_js_runtime.dll"
		default:
			panic("unreachable")
		}
	}()

	var (
		cargoTarget     string
		extraEnvs       []string
		useZig          bool
		zigGlibcVersion string
	)

	// Add the type def path if requested.
	if napi, ok := in.NapiTypeDefPath.Get(); ok {
		extraEnvs = append(extraEnvs, "TYPE_DEF_TMP_PATH="+napi.ToIO())
	}

	extraEnvs = append(extraEnvs, fmt.Sprintf("ENCORE_VERSION=%s", in.Version))

	switch in.Target.OS {
	case Darwin:
		switch in.Target.Arch {
		case Amd64:
			cargoTarget = "x86_64-apple-darwin"
		case Arm64:
			cargoTarget = "aarch64-apple-darwin"
		default:
			return nil, errors.Newf("unsupported architecture for darwin: %q", in.Target.Arch)
		}

		// A macOS host builds either architecture with the Xcode toolchain;
		// cross-compiling from another OS needs zig and the SDK.
		if runtime.GOOS != "darwin" {
			crossPath, ok := in.CrossMacSDKPath.Get()
			if !ok {
				return nil, errors.New("macOS SDK path must be set for cross-compiles to macOS")
			}
			extraEnvs = append(extraEnvs, "SDKROOT="+crossPath.ToIO())
			useZig = true
		}

	case Linux:
		switch in.Target.Arch {
		case Amd64:
			cargoTarget = "x86_64-unknown-linux-gnu"
		case Arm64:
			cargoTarget = "aarch64-unknown-linux-gnu"
		default:
			return nil, errors.Newf("unsupported architecture for linux: %q", in.Target.Arch)
		}

		useZig = in.Target.IsCross() || in.ForDistribution
		zigGlibcVersion = ".2.31"

	case Windows:
		if in.Target.IsCross() {
			return nil, errors.Newf("cross-compiling to windows is not supported")
		} else if in.Target.Arch != Amd64 {
			return nil, errors.Newf("unsupported architecture for windows: %q", in.Target.Arch)
		}

		// Must use msvc for napi.
		cargoTarget = "x86_64-pc-windows-msvc"

	default:
		panic("unreachable")
	}

	out, err := rustbuild.Compile(ctx, rustbuild.CompileInput{
		PackageRoot:     in.Host.RepoPath.Join("runtimes", "js"),
		CargoTargetDir:  in.CargoTargetDir,
		ReleaseBuild:    in.ReleaseBuild,
		ArtifactName:    artifactName,
		CargoTarget:     cargoTarget,
		UseZig:          useZig,
		ZigGlibcVersion: zigGlibcVersion,
		ExtraEnvs:       extraEnvs,
	})
	if err != nil {
		return nil, err
	}
	checksumFile, err := WriteSha256sum(out.Artifact)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write checksum")
	}
	return &CompileNativeModuleOutput{
		NativeModule: out.Artifact,
		Checksum:     checksumFile,
	}, nil
}
