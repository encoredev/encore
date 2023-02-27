package build

import (
	"context"
	"go/token"
	"os"
	"runtime"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"encr.dev/v2/compiler/build"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	perr2 "encr.dev/v2/internal/perr"
)

var Cmd = &cobra.Command{
	Use:   "build",
	Short: "Build an Encore service.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}

		ctx := context.Background()
		fs := token.NewFileSet()
		errs := perr2.NewList(ctx, fs)
		pc := &parsectx.Context{
			Ctx: ctx,
			Log: zerolog.New(zerolog.NewConsoleWriter()),
			Build: parsectx.BuildInfo{
				GOARCH: runtime.GOARCH,
				GOOS:   runtime.GOOS,
				GOROOT: paths.RootedFSPath(runtime.GOROOT(), "."),
				// TODO(andre) hack
				EncoreRuntime: paths.RootedFSPath(os.Getenv("ENCORE_RUNTIME_PATH"), "."),
				BuildTags:     nil,
				CgoEnabled:    false,
				StaticLink:    false,
				Debug:         false,
			},
			MainModuleDir: paths.RootedFSPath(wd, "."),
			FS:            fs,
			ParseTests:    false,
			Errs:          errs,
		}

		res := build.Build(&build.Config{
			Ctx:        pc,
			Overlays:   nil, // TODO add
			MainPkg:    paths.MustPkgPath(args[0]),
			KeepOutput: true,
		})
		if errs.Len() > 0 {
			pc.Log.Fatal().Msg(errs.FormatErrors())
		}
		pc.Log.Info().Msgf("got result %+v", *res)
	},
}
