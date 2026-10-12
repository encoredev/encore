package supervisor

import (
	"context"
	"fmt"
	"runtime"

	"encr.dev/pkg/releaser/steps/rustbuild"
	"github.com/cockroachdb/errors"

	"encr.dev/pkg/option"
	. "encr.dev/pkg/releaser/bu"
)

type CompileInput struct {
	Host   HostInfo
	Target Platform

	// The cargo 'target' directory to use for compiled outputs.
	CargoTargetDir FSPath

	// The path to the macOS SDK. Only needed when cross-compiling to macOS
	// from another OS; native macOS builds use the Xcode toolchain.
	CrossMacSDKPath option.Option[FSPath]

	// Version string to embed in the binary.
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

type CompileOutput struct {
	// Path to the built supervisor binary.
	SupervisorBinary   FSPath
	SupervisorChecksum FSPath
}

// Compile compiles the supervisor binary.
func Compile(ctx context.Context, in CompileInput) (*CompileOutput, error) {
	artifactName := "supervisor-encore"
	if in.Target.OS == Windows {
		artifactName += ".exe"
	}

	var (
		cargoTarget     string
		extraEnvs       []string
		useZig          bool
		zigGlibcVersion string
	)

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
			cargoTarget = "x86_64-unknown-linux-musl"
		case Arm64:
			cargoTarget = "aarch64-unknown-linux-musl"
		default:
			return nil, errors.Newf("unsupported architecture for linux: %q", in.Target.Arch)
		}

		useZig = in.Target.IsCross() || in.ForDistribution
	case Windows:
		switch in.Target.Arch {
		case Amd64:
			cargoTarget = "x86_64-pc-windows-msvc"
		default:
			return nil, errors.Newf("unsupported architecture for windows: %q", in.Target.Arch)
		}

	default:
		return nil, errors.Newf("unsupported OS: %q", in.Target.OS)
	}

	out, err := rustbuild.Compile(ctx, rustbuild.CompileInput{
		PackageRoot:     in.Host.RepoPath.Join("supervisor"),
		CargoTargetDir:  in.CargoTargetDir,
		ReleaseBuild:    in.ReleaseBuild,
		ArtifactName:    artifactName,
		CargoTarget:     cargoTarget,
		UseZig:          useZig,
		ZigGlibcVersion: zigGlibcVersion,
		ExtraEnvs:       extraEnvs,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile supervisor")
	}

	checksumFile, err := WriteSha256sum(out.Artifact)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create checksum")
	}

	return &CompileOutput{
		SupervisorBinary:   out.Artifact,
		SupervisorChecksum: checksumFile,
	}, nil
}
