package compile

import (
	osPkg "os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"encr.dev/pkg/encorebuild/buildconf"
	. "encr.dev/pkg/encorebuild/buildutil"
)

// GoBinary compiles a Go binary for the given OS and architecture with GCP enabled
//
// This file was inspired by the blog post: https://lucor.dev/post/cross-compile-golang-fyne-project-using-zig/
func GoBinary(cfg *buildconf.Config, outputPath string, entrypointPkg string, ldFlags []string) {
	cc, cxx, compilerEnvs, compilerLDFlags := compilerSettings(cfg)
	if cfg.OS == "windows" && !strings.HasSuffix(outputPath, ".exe") {
		outputPath += ".exe"
	}

	combinedLDFlags := append(append([]string{}, compilerLDFlags...), ldFlags...)

	baseEnvs := goBaseEnvs(cfg)
	envs := append(baseEnvs,
		"CGO_ENABLED=1",
		"CC="+cc,
		"CXX="+cxx,
	)
	envs = append(envs, compilerEnvs...)

	// Build the go build args
	args := []string{"build",
		"-trimpath",
		"-tags", "netgo", // Always force netgo otherwise we end up with segfaults on MacOS
	}
	if len(combinedLDFlags) > 0 {
		args = append(args, "-ldflags="+strings.Join(combinedLDFlags, " "))
	}
	if cfg.OS == "darwin" {
		args = append(args, "-buildmode=pie")
	}
	args = append(args,
		"-o", outputPath,
		entrypointPkg,
	)

	// Build the command
	cmd := exec.Command("go", args...)
	cmd.Env = envs

	// nosemgrep
	if out, err := cmd.CombinedOutput(); err != nil {
		Bailf("failed to compile go binary: %v: %s", err, string(out))
	}
}

func goBaseEnvs(cfg *buildconf.Config) []string {
	gocache := filepath.Join(cfg.CacheDir, "go", cfg.OS, cfg.Arch)
	Check(osPkg.MkdirAll(gocache, 0755))
	return append(osPkg.Environ(),
		"GOOS="+cfg.OS,
		"GOARCH="+cfg.Arch,
		"GOCACHE="+gocache,
	)
}

// CompileRustBinary compiles a Rust binary for the given OS and architecture
//
// We're using zigbuild to perform easy cross compiling
func RustBinary(cfg *buildconf.Config, artifactPath, outputPath string, cratePath string, libc string, extraEnvVars ...string) {
	if cfg.OS == "windows" {
		if !strings.HasSuffix(artifactPath, ".dll") {
			outputPath += ".exe"
			artifactPath += ".exe"
		}
	}

	envs := append(extraEnvVars, osPkg.Environ()...)
	useCross := false
	if cfg.IsCross() && runtime.GOOS == "darwin" {
		// check is cross is installed
		_, err := exec.LookPath("cross")
		if err == nil {
			useCross = true
		}
	}

	useZig := !useCross

	var target, zigTargetSuffix string
	switch cfg.OS {
	case "darwin":
		switch cfg.Arch {
		case "amd64":
			target = "x86_64-apple-darwin"
		case "arm64":
			target = "aarch64-apple-darwin"
		default:
			Bailf("unsupported architecture for darwin: %q", cfg.Arch)
		}

		// We need to set the SDKROOT for cross compiling to MacOS
		if cfg.IsCross() {
			envs = append(envs, "SDKROOT="+cfg.CrossMacSDKPath())
		}

	case "linux":
		switch cfg.Arch {
		case "amd64":
			target = "x86_64-unknown-linux-" + libc
		case "arm64":
			target = "aarch64-unknown-linux-" + libc
		default:
			Bailf("unsupported architecture for linux: %q", cfg.Arch)
		}

		// If we're using zig, specify the glibc version we want.
		if useZig {
			zigTargetSuffix = ".2.31"
		}

	case "windows":
		switch cfg.Arch {
		case "amd64":
			target = "x86_64-pc-windows-gnu"
		default:
			Bailf("unsupported architecture for windows: %q", cfg.Arch)
		}

	default:
		Bailf("unsupported os: %q", cfg.OS)
	}

	// Create a cache dir for the go build cache for this specific OS and architecture pair
	path := filepath.Join(cfg.CacheDir, "rust", cfg.OS, cfg.Arch)

	Check(osPkg.MkdirAll(path, 0755))

	// Build the command
	cargoArgs := []string{
		"build",
		"--target", target + zigTargetSuffix,
		"--target-dir", path,
	}
	if useZig {
		cargoArgs[0] = "zigbuild"
	}
	buildMode := "debug"
	if cfg.Release {
		cargoArgs = append(cargoArgs, "--release")
		buildMode = "release"
	}

	builder := "cargo"
	if useCross {
		builder = "cross"
	}
	cmd := exec.Command(builder, cargoArgs...)
	// forwards the output to the parent process
	cmd.Stdout = osPkg.Stdout
	cmd.Stderr = osPkg.Stderr
	cmd.Dir = cratePath
	cmd.Env = envs

	// Cargo can't run multiple compiles at the same time for the same crate
	// so let's lock here, then unlock once the compile has finished
	cargoLock.Lock()
	defer cargoLock.Unlock()

	// nosemgrep
	if err := cmd.Run(); err != nil {
		Bailf("failed to compile rust binary: %v", err)
	}

	// Copy the binary to the output path
	binaryFile := filepath.Join(path, target, buildMode, artifactPath)
	cmd = exec.Command("cp", binaryFile, outputPath)
	// nosemgrep
	if out, err := cmd.CombinedOutput(); err != nil {
		Bailf("failed to copy rust binary: %v: %s", err, string(out))
	}
}

// compilerSettings returns the CC and CXX settings for the given OS and architecture
func compilerSettings(cfg *buildconf.Config) (cc, cxx string, envs, ldFlags []string) {
	var zigTarget string
	var zigArgs string
	zigBinary := "zig"

	switch cfg.OS {
	case "darwin":
		zigBinary = "/usr/local/zig-0.9.1/zig" // We need an explicit version of Zig for darwin (0.11.0 compiles, build causes runtime errors)
		ldFlags = []string{"-s", "-w"}

		switch cfg.Arch {
		case "amd64":
			zigTarget = "x86_64-macos.10.12"
		case "arm64":
			zigTarget = "aarch64-macos.11.1"
		default:
			Bailf("unsupported architecture for darwin: %q", cfg.Arch)
		}

		// We need to set some extra stuff if we're cross compiling to MacOS
		if cfg.IsCross() {
			sdkPath := cfg.CrossMacSDKPath()
			zigArgs = " -isysroot " + sdkPath + " -iwithsysroot /usr/include -iframeworkwithsysroot /System/Library/Frameworks"
			envs = []string{
				"CGO_LDFLAGS=--sysroot " + sdkPath + " -F/System/Library/Frameworks -L/usr/lib",
			}
		}

	case "linux":
		switch cfg.Arch {
		case "amd64":
			// Note: we're not targeting a specific glibc version here as we tried before
			// with 2.35 - but for some reason we still get runtime errors not finding 2.34 or 2.33 on WSL (which had 2.35)
			zigTarget = "x86_64-linux-gnu.2.31"
			zigArgs = " -static -isystem /usr/include"
		case "arm64":
			zigTarget = "aarch64-linux-gnu.2.31"
			zigArgs = " -static -isystem /usr/include"
			envs = []string{
				"PKG_CONFIG_LIBDIR=/usr/lib/aarch64-linux-gnu/pkgconfig",
			}
		default:
			Bailf("unsupported architecture for linux: %q", cfg.Arch)
		}

	case "windows":
		switch cfg.Arch {
		case "amd64":
			zigTarget = "x86_64-windows-gnu"
		default:
			Bailf("unsupported architecture for windows: %q", cfg.Arch)
		}

		ldFlags = []string{"-H=windowsgui"}

	default:
		Bailf("unsupported os: %q", cfg.OS)
	}

	cc = zigBinary + " cc -target " + zigTarget + zigArgs
	cxx = zigBinary + " c++ -target " + zigTarget + zigArgs
	return cc, cxx, envs, ldFlags
}

// cargoLock is a lock to prevent concurrent cargo builds.
var cargoLock sync.Mutex
