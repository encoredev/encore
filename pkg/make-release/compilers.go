package main

import (
	osPkg "os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
)

// MacOSSDKPath is the path to where the MacOS SDK is located on Encore's builder systems
const MacOSSDKPath = "/sdk"

func GoBaseEnvs(os string, arch string) ([]string, error) {
	// Create a cache dir for the go build cache for this specific OS and architecture pair
	cacheDir, err := osPkg.UserCacheDir()
	if err != nil {
		return nil, errors.Wrap(err, "user cache dir")
	}

	path := filepath.Join(cacheDir, "encore-build-cache", "go", os, arch)

	err = osPkg.MkdirAll(path, 0755)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make cache dir")
	}

	return append(osPkg.Environ(),
		"GOOS="+os,
		"GOARCH="+arch,
		"GOCACHE="+path,
	), nil
}

// CompileGoBinary compiles a Go binary for the given OS and architecture with GCP enabled
//
// This file was inspired by the blog post: https://lucor.dev/post/cross-compile-golang-fyne-project-using-zig/
func CompileGoBinary(outputPath string, entrypointPkg string, ldFlags []string, os string, arch string) error {
	cc, cxx, compilerEnvs, compilerLDFlags, err := compilerSettings(os, arch)
	if err != nil {
		return errors.Wrap(err, "compiler settings")
	}

	if os == "windows" {
		outputPath += ".exe"
	}

	combinedLDFlags := append(append([]string{}, compilerLDFlags...), ldFlags...)

	baseEnvs, err := GoBaseEnvs(os, arch)
	if err != nil {
		return errors.Wrap(err, "go base envs")
	}

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
	if os == "darwin" {
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Newf("failed to compile go binary: %s", string(out))
	}

	return nil
}

// CompileRustBinary compiles a Rust binary for the given OS and architecture
//
// We're using zigbuild to perform easy cross compiling
func CompileRustBinary(artifactPath, outputPath string, cratePath string, os string, arch string, extraEnvVars ...string) error {
	if os == "windows" {
		if !strings.HasSuffix(artifactPath, ".dll") {
			outputPath += ".exe"
			artifactPath += ".exe"
		}
	}

	envs := append(extraEnvVars, osPkg.Environ()...)

	var target string
	switch os {
	case "darwin":
		switch arch {
		case "amd64":
			target = "x86_64-apple-darwin"
		case "arm64":
			target = "aarch64-apple-darwin"
		default:
			return errors.New("unsupported architecture for darwin: " + arch)
		}

		// We need to set the SDKROOT for cross compiling to MacOS
		if runtime.GOOS != "darwin" {
			envs = append(envs,
				"SDKROOT="+MacOSSDKPath,
			)
		}

	case "linux":
		switch arch {
		case "amd64":
			target = "x86_64-unknown-linux-gnu"
		case "arm64":
			target = "aarch64-unknown-linux-gnu"
		default:
			return errors.New("unsupported architecture for linux: " + arch)
		}

	case "windows":
		switch arch {
		case "amd64":
			target = "x86_64-pc-windows-gnu"
		default:
			return errors.New("unsupported architecture for windows: " + arch)
		}

	default:
		return errors.New("unsupported os: " + os)
	}

	// Create a cache dir for the go build cache for this specific OS and architecture pair
	cacheDir, err := osPkg.UserCacheDir()
	if err != nil {
		return errors.Wrap(err, "user cache dir")
	}

	path := filepath.Join(cacheDir, "encore-build-cache", "rust", os, arch)

	err = osPkg.MkdirAll(path, 0755)
	if err != nil {
		return errors.Wrap(err, "failed to make cache dir")
	}

	// Build the command
	cargoArgs := []string{"zigbuild",
		"--target", target,
		"--target-dir", path,
		"--release",
	}

	cmd := exec.Command("cargo", cargoArgs...)
	cmd.Dir = cratePath
	cmd.Env = envs

	// Cargo can't run multiple compiles at the same time for the same crate
	// so let's lock here, then unlock once the compile has finished
	cargoLock.Lock()
	defer cargoLock.Unlock()

	// nosemgrep
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Newf("failed to compile rust binary: %v - %s", err, string(out))
	}

	// Copy the binary to the output path
	binaryFile := filepath.Join(path, target, "release", artifactPath)
	cmd = exec.Command("cp", binaryFile, outputPath)
	// nosemgrep
	out, err = cmd.CombinedOutput()
	if err != nil {
		return errors.Newf("failed to copy rust binary: %v - %s", err, string(out))
	}

	return nil
}

// compilerSettings returns the CC and CXX settings for the given OS and architecture
func compilerSettings(os string, arch string) (cc, cxx string, envs, ldFlags []string, err error) {
	var zigTarget string
	var zigArgs string
	zigBinary := "zig" // pick it up off the path

	switch os {
	case "darwin":
		zigBinary = "/usr/local/zig-0.9.1/zig" // We need an explicit version of Zig for darwin (0.11.0 compiles, build causes runtime errors)
		ldFlags = []string{"-s", "-w"}

		switch arch {
		case "amd64":
			zigTarget = "x86_64-macos.10.12"
		case "arm64":
			zigTarget = "aarch64-macos.11.1"
		default:
			return "", "", nil, nil, errors.New("unsupported architecture for darwin: " + arch)
		}

		// We need to set some extra stuff if we're cross compiling to MacOS
		if runtime.GOOS != "darwin" {
			zigArgs = " -isysroot " + MacOSSDKPath + " -iwithsysroot /usr/include -iframeworkwithsysroot /System/Library/Frameworks"
			envs = []string{
				"CGO_LDFLAGS=--sysroot " + MacOSSDKPath + " -F/System/Library/Frameworks -L/usr/lib",
			}
		}

	case "linux":
		switch arch {
		case "amd64":
			zigTarget = "x86_64-linux-gnu" // Note: we're not targeting a specific glibc version here as we tried before with 2.35 - but for some reason we still get runtime errors not finding 2.34 or 2.33 on WSL (which had 2.35)
			zigArgs = " -static -isystem /usr/include"
		case "arm64":
			zigTarget = "aarch64-linux-gnu"
			zigArgs = " -static -isystem /usr/include"
			envs = []string{
				"PKG_CONFIG_LIBDIR=/usr/lib/aarch64-linux-gnu/pkgconfig",
			}
		default:
			return "", "", nil, nil, errors.New("unsupported architecture for linux: " + arch)
		}

	case "windows":
		switch arch {
		case "amd64":
			zigTarget = "x86_64-windows-gnu"
		default:
			return "", "", nil, nil, errors.New("unsupported architecture for windows: " + arch)
		}

		ldFlags = []string{"-H=windowsgui"}

	default:
		return "", "", nil, nil, errors.New("unsupported os: " + os)
	}

	cc = zigBinary + " cc -target " + zigTarget + zigArgs
	cxx = zigBinary + " c++ -target " + zigTarget + zigArgs
	return cc, cxx, envs, ldFlags, nil
}

var (
	cargoLock sync.Mutex
)
