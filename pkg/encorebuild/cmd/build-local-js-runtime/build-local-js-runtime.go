package main

import (
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

func main() {
	log.Logger = zerolog.New(zerolog.NewConsoleWriter()).With().Caller().Timestamp().Stack().Logger()

	root, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get working directory")
	} else if _, err := os.Stat(join(root, ".git")); err != nil {
		log.Fatal().Err(err).Msg("expected to run build-local-js-runtime from encr.dev repository root")
	}

	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get user cache dir")
	}
	cacheDir := filepath.Join(userCacheDir, "encore-build-cache")

	cfg := &buildconf.Config{
		Log:                    log.Logger,
		OS:                     runtime.GOOS,
		Arch:                   runtime.GOARCH,
		Release:                false,
		Version:                version.Version,
		RepoDir:                root,
		CacheDir:               cacheDir,
		MacSDKPath:             option.None[string](),
		CopyNativeModuleToRepo: true,
	}
	encorebuild.BuildJSRuntime(cfg)
}
