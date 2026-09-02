package gobuild

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"encr.dev/internal/version"
	"encr.dev/pkg/option"
	. "encr.dev/pkg/releaser/bu"
	"github.com/cockroachdb/errors"
)

type ExeInput struct {
	Host   HostInfo
	Target Platform

	// The path to the MacOS SDK. Must be set for cross-compiles to macOS.
	CrossMacSDKPath option.Option[FSPath]

	// Version string to embed in the binary.
	Version string

	// Where to invoke the 'go build' command from.
	WorkingDir FSPath

	// The main package to build, relative to the working directory.
	MainPkg string

	// Output exe path.
	OutExePath FSPath
}

// Exe compiles a Go binary.
func Exe(ctx context.Context, in ExeInput) error {
	var artifactName string
	if in.MainPkg != "" && in.MainPkg != "." {
		artifactName = path.Base(in.MainPkg)
	}
	if artifactName == "" {
		artifactName = filepath.Base(string(in.WorkingDir))
	}
	if in.Target.OS == Windows && !strings.HasSuffix(artifactName, ".exe") {
		artifactName += ".exe"
	}

	extraLdFlags := []string{
		"-X", fmt.Sprintf("'encr.dev/internal/version.Version=%s'", in.Version),
	}

	// If we're building a nightly, devel or beta version, we need to set the default config directory
	{
		var versionSuffix string
		switch version.ChannelFor(in.Version) {
		case version.GA:
			versionSuffix = ""
		case version.Beta:
			versionSuffix = "-beta"
		case version.Nightly:
			versionSuffix = "-nightly"
		case version.DevBuild:
			versionSuffix = "-develop"
		default:
			return errors.Newf("unknown version channel for %s", in.Version)
		}
		if versionSuffix != "" {
			extraLdFlags = append(extraLdFlags,
				"-X", "'encr.dev/internal/conf.defaultConfigDirectory=encore"+versionSuffix+"'",
			)
		}
	}

	err := Compile(ctx, CompileInput{
		Target:            in.Target,
		WorkingDir:        in.Host.RepoPath,
		MainPkg:           in.MainPkg,
		CrossMacSDKPath:   in.CrossMacSDKPath,
		OutExecutablePath: in.OutExePath,
		ExtraLdFlags:      extraLdFlags,
	})
	return errors.Wrapf(err, "unable to compile %s for %s", artifactName, in.Target)
}
