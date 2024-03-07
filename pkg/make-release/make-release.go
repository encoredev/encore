package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encr.dev/internal/version"
)

func join(strs ...string) string {
	return filepath.Join(strs...)
}

func main() {
	log.Logger = zerolog.New(zerolog.NewConsoleWriter()).With().Caller().Timestamp().Stack().Logger()

	dst := flag.String("dst", "", "build destination")
	versionStr := flag.String("v", "", "version number")
	tsParserRepo := flag.String("ts-parser", "", "path to ts-parser repo")
	onlyBuild := flag.String("only", "", "build only the valid target ('darwin-arm64' or 'darwin' or 'arm64' or '' for all)")
	flag.Parse()
	if *dst == "" || *versionStr == "" || *tsParserRepo == "" {
		log.Fatal().Msgf("missing -dst %q, -v %q or ts-parser %q", *dst, *versionStr, *tsParserRepo)
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

	jsBuilder := &JSPackager{
		WorkspaceRoot:    join(root, "runtimes", "js"),
		Version:          *versionStr,
		log:              log.Logger.With().Str("builder", "js").Logger(),
		compileCompleted: make(chan struct{}),
	}

	// Create all the builders
	builders := []*DistBuilder{
		{OS: "darwin", Arch: "amd64"},
		{OS: "darwin", Arch: "arm64"},
		{OS: "linux", Arch: "amd64"},
		{OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"},
	}
	parralelFuncs := make([]func() error, 1, len(builders)+1)
	parralelFuncs[0] = jsBuilder.Package

	// Give them the common settings
	for _, b := range builders {
		if *onlyBuild != "" && !(*onlyBuild == fmt.Sprintf("%s-%s", b.OS, b.Arch) ||
			*onlyBuild == b.OS ||
			*onlyBuild == b.Arch) {
			continue
		}
		b.TSParserPath = *tsParserRepo
		b.DistBuildDir = join(*dst, b.OS+"_"+b.Arch)
		b.ArtifactsTarFile = join(*dst, "artifacts", "encore-"+*versionStr+"-"+b.OS+"_"+b.Arch+".tar.gz")
		b.Version = *versionStr
		b.jsBuilder = jsBuilder

		parralelFuncs = append(parralelFuncs, b.Build)
	}

	if err := runParallel(parralelFuncs...); err != nil {
		log.Fatal().Err(err).Msg("failed to build all distributions")
	}
	log.Info().Msg("all distributions built successfully")
}
