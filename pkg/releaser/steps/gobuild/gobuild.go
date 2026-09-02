package gobuild

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cockroachdb/errors"

	"encr.dev/pkg/option"
	. "encr.dev/pkg/releaser/bu"
)

type CompileInput struct {
	Target Platform

	// The working directory for the build.
	WorkingDir FSPath

	// Argument for the main package.
	MainPkg string

	// The path to the macOS SDK. Only needed when cross-compiling to macOS
	// from another OS; native macOS builds use the Xcode toolchain.
	CrossMacSDKPath option.Option[FSPath]

	// The output path for the compiled executable.
	OutExecutablePath FSPath

	// Extra flags to pass to the linker.
	ExtraLdFlags []string
}

// Compile compiles a Go program.
func Compile(ctx context.Context, in CompileInput) error {
	cc, cxx, compilerEnvs, compilerLDFlags, err := compilerSettings(&in)
	if err != nil {
		return err
	}

	combinedLDFlags := append(append([]string{}, compilerLDFlags...), in.ExtraLdFlags...)

	envs := []string{
		"GOOS=" + in.Target.OS.String(),
		"GOARCH=" + in.Target.Arch.String(),
		"CGO_ENABLED=1",
		"CC=" + cc,
		"CXX=" + cxx,
	}
	envs = append(envs, compilerEnvs...)

	// Build the go build args
	args := []string{"build",
		"-trimpath",
		"-tags", "netgo", // Always force netgo otherwise we end up with segfaults on MacOS
	}
	if len(combinedLDFlags) > 0 {
		args = append(args, "-ldflags="+strings.Join(combinedLDFlags, " "))
	}
	if in.Target.OS == Darwin {
		args = append(args, "-buildmode=pie")
	}
	args = append(args,
		"-o", in.OutExecutablePath.ToIO(),
		in.MainPkg,
	)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = append(os.Environ(), envs...)
	cmd.Dir = in.WorkingDir.ToIO()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to compile go binary")
	}

	return nil
}

// compilerSettings returns the C compiler settings for the target.
//
// Native macOS builds use Apple clang from the Xcode command line tools,
// which covers both darwin architectures (cgo passes -arch from GOARCH).
// Everything else compiles with zig cc, which bundles the target's libc:
// glibc pinned to 2.31 on Linux so the binaries run on older distributions
// than the build host, mingw-w64 on Windows, and — only when
// cross-compiling to macOS from another OS — the macOS SDK given in
// CrossMacSDKPath. The release workflow installs the pinned zig version.
func compilerSettings(cfg *CompileInput) (cc, cxx string, envs, ldFlags []string, err error) {
	var zigTarget string
	var zigArgs string

	switch cfg.Target.OS {
	case Darwin:
		ldFlags = []string{"-w"}

		var minOSVersion string
		switch cfg.Target.Arch {
		case Amd64:
			zigTarget = "x86_64-macos.10.12"
			minOSVersion = "10.12"
		case Arm64:
			zigTarget = "aarch64-macos.11.1"
			minOSVersion = "11.1"
		default:
			return "", "", nil, nil, errors.Newf("unsupported architecture for darwin: %q", cfg.Target.Arch)
		}

		if runtime.GOOS == "darwin" {
			envs = []string{"MACOSX_DEPLOYMENT_TARGET=" + minOSVersion}
			return "clang", "clang++", envs, ldFlags, nil
		}

		// Cross-compiling from another OS needs the SDK for the system
		// headers and frameworks.
		sdkPath, ok := cfg.CrossMacSDKPath.Get()
		if !ok {
			return "", "", nil, nil, errors.New("macOS SDK path must be set for cross-compiles to macOS")
		}
		zigArgs = " -isysroot " + sdkPath.ToIO() + " -iwithsysroot /usr/include -iframeworkwithsysroot /System/Library/Frameworks"
		envs = []string{
			"CGO_LDFLAGS=--sysroot " + sdkPath.ToIO() + " -F/System/Library/Frameworks -L/usr/lib",
		}

	case Linux:
		// Note: we're not targeting a newer glibc here as we tried before
		// with 2.35 - but for some reason we still got runtime errors not finding 2.34 or 2.33 on WSL (which had 2.35)
		switch cfg.Target.Arch {
		case Amd64:
			zigTarget = "x86_64-linux-gnu.2.31"
		case Arm64:
			zigTarget = "aarch64-linux-gnu.2.31"
			if cfg.Target.IsCross() {
				// Point pkg-config at the multiarch arm64 libraries when
				// cross-compiling from an amd64 host.
				envs = []string{"PKG_CONFIG_LIBDIR=/usr/lib/aarch64-linux-gnu/pkgconfig"}
			}
		default:
			return "", "", nil, nil, errors.Newf("unsupported architecture for linux: %q", cfg.Target.Arch)
		}
		zigArgs = " -static -isystem /usr/include"

	case Windows:
		switch cfg.Target.Arch {
		case Amd64:
			zigTarget = "x86_64-windows-gnu"
		default:
			return "", "", nil, nil, errors.Newf("unsupported architecture for windows: %q", cfg.Target.Arch)
		}

		ldFlags = []string{"-H=windowsgui"}

	default:
		panic("unreachable")
	}

	// Resolve through PATH at build time (cmd/go does the same for CC),
	// but fail early with a clear message if zig is missing.
	if _, err := exec.LookPath("zig"); err != nil {
		return "", "", nil, nil, errors.Wrap(err, "zig not found in PATH")
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", "", nil, nil, errors.Wrap(err, "failed to get user cache dir")
	}
	zigCacheDir := filepath.Join(cacheDir, "gozig", cfg.Target.OS.String(), cfg.Target.Arch.String())
	envs = append(envs,
		"ZIG_LOCAL_CACHE_DIR="+zigCacheDir,
		"ZIG_GLOBAL_CACHE_DIR="+zigCacheDir,
	)

	cc = "zig cc -target " + zigTarget + zigArgs
	cxx = "zig c++ -target " + zigTarget + zigArgs
	return cc, cxx, envs, ldFlags, nil
}
