package buildconf

import (
	"runtime"

	"encr.dev/pkg/encorebuild/buildutil"
	"encr.dev/pkg/option"
	"github.com/rs/zerolog"
)

type Config struct {
	// Logger to use.
	Log zerolog.Logger

	// Target OS and architecture (in GOOS/GOARCH format)
	OS   string
	Arch string

	// Release is true if this is a release build.
	Release bool

	// The version being built.
	Version string

	// RepoDir is the path to the encore repo on the filesystem.
	RepoDir string

	// CacheDir is the cache dir to use for the build.
	CacheDir string

	// The path to the MacOS SDK. Must be set for cross-compiles to macOS.
	MacSDKPath option.Option[string]
}

// IsCross reports whether the build is a cross-compile.
func (cfg *Config) IsCross() bool {
	return cfg.OS != runtime.GOOS || cfg.Arch != runtime.GOARCH
}

func (cfg *Config) CrossMacSDKPath() string {
	if cfg.OS != "darwin" {
		return ""
	}
	val, ok := cfg.MacSDKPath.Get()
	if !ok {
		buildutil.Bailf("macOS SDK path must be set for cross-compiles to macOS")
	}
	return val
}

// Exe returns the executable file suffix for the target OS.
func (cfg *Config) Exe() string {
	if cfg.OS == "windows" {
		return ".exe"
	}
	return ""
}
