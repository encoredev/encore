package main

import (
	"flag"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encr.dev/internal/version"
	"encr.dev/pkg/encorebuild"
	"encr.dev/pkg/encorebuild/buildconf"
	"encr.dev/pkg/option"
)

func join(strs ...string) string {
	return filepath.Join(strs...)
}

var archFlag = flag.String("arch", runtime.GOARCH, "the architecture to target")
var osFlag = flag.String("os", runtime.GOOS, "the operating system to target")

func main() {
	binary := os.Args[1]
	if binary == "" || binary[0] == '-' {
		log.Fatal().Msg("expected binary name as first argument")
	}
	os.Args = os.Args[1:]
	flag.Parse()
	log.Logger = zerolog.New(zerolog.NewConsoleWriter()).With().Caller().Timestamp().Stack().Logger()

	root, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get working directory")
	} else if _, err := os.Stat(join(root, ".git")); err != nil {
		log.Fatal().Err(err).Msg("expected to run build-local-binary from encr.dev repository root")
	}

	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get user cache dir")
	}
	cacheDir := filepath.Join(userCacheDir, "encore-build-cache")

	cfg := &buildconf.Config{
		Log:        log.Logger,
		OS:         *osFlag,
		Arch:       *archFlag,
		Release:    false,
		Version:    version.Version,
		RepoDir:    root,
		CacheDir:   cacheDir,
		MacSDKPath: option.None[string](),
		CopyToRepo: true,
	}

	switch binary {
	case "all":
		encorebuild.NewJSRuntimeBuilder(cfg).Build()
		encorebuild.NewSupervisorBuilder(cfg).Build()
	case "supervisor-encore":
		encorebuild.NewSupervisorBuilder(cfg).Build()
	case "encore-runtime.node":
		encorebuild.NewJSRuntimeBuilder(cfg).Build()
	default:
		log.Fatal().Msgf("unknown binary %s", binary)
	}
}
