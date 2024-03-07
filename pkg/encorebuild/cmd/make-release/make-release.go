package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encr.dev/internal/version"
	"encr.dev/pkg/encorebuild"
	"encr.dev/pkg/encorebuild/buildconf"
	"encr.dev/pkg/encorebuild/buildutil"
	"encr.dev/pkg/option"
)

func join(strs ...string) string {
	return filepath.Join(strs...)
}

func main() {
	log.Logger = zerolog.New(zerolog.NewConsoleWriter()).With().Caller().Timestamp().Stack().Logger()

	dst := flag.String("dst", "", "build destination")
	versionStr := flag.String("v", "", "version number")
	onlyBuild := flag.String("only", "", "build only the valid target ('darwin-arm64' or 'darwin' or 'arm64' or '' for all)")
	flag.Parse()
	if *dst == "" || *versionStr == "" {
		log.Fatal().Msgf("missing -dst %q or -v %q", *dst, *versionStr)
	}

	if (*versionStr)[0] != 'v' {
		log.Fatal().Msg("version must start with 'v'")
	}
	switch version.ChannelFor(*versionStr) {
	case version.GA, version.Beta, version.Nightly, version.DevBuild:
		// no-op
	default:
		log.Fatal().Msgf("unknown version channel for %s", *versionStr)
	}

	root, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get working directory")
	} else if _, err := os.Stat(join(root, "go.mod")); err != nil {
		log.Fatal().Err(err).Msg("expected to run make-release.go from encr.dev repository root")
	}

	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get user cache dir")
	}
	cacheDir := filepath.Join(userCacheDir, "encore-build-cache")

	*dst, err = filepath.Abs(*dst)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get absolute path to destination")
	}

	// Prepare the target directory.
	if err := os.RemoveAll(*dst); err != nil {
		log.Fatal().Err(err).Msg("failed to remove existing target dir")
	} else if err := os.MkdirAll(filepath.Join(*dst, "artifacts"), 0755); err != nil {
		log.Fatal().Err(err).Msg("failed to create target dir")
	}

	type buildTarget struct {
		OS   string
		Arch string
	}

	targets := []buildTarget{
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"windows", "amd64"},
	}

	var parralelFuncs []func()
	// Give them the common settings
	for _, t := range targets {
		if *onlyBuild != "" && !(*onlyBuild == fmt.Sprintf("%s-%s", t.OS, t.Arch) ||
			*onlyBuild == t.OS ||
			*onlyBuild == t.Arch) {
			continue
		}

		b := &encorebuild.DistBuilder{
			Cfg: &buildconf.Config{
				Log:        log.With().Str("os", t.OS).Str("arch", t.Arch).Logger(),
				OS:         t.OS,
				Arch:       t.Arch,
				Release:    true,
				Version:    *versionStr,
				RepoDir:    root,
				CacheDir:   cacheDir,
				MacSDKPath: option.Some("/sdk"),
			},
			DistBuildDir:     join(*dst, t.OS+"_"+t.Arch),
			ArtifactsTarFile: join(*dst, "artifacts", "encore-"+*versionStr+"-"+t.OS+"_"+t.Arch+".tar.gz"),
		}

		parralelFuncs = append(parralelFuncs, b.Build)
	}

	defer func() {
		if err := recover(); err != nil {
			if b, ok := err.(buildutil.Bailout); ok {
				log.Fatal().Err(b.Err).Msg("failed to build")
			} else {
				stack := debug.Stack()
				log.Fatal().Msgf("failed to build: unrecovered panic: %v: \n%s", err, stack)
			}
		}
	}()

	buildutil.RunParallel(parralelFuncs...)
	log.Info().Msg("all distributions built successfully")
}
