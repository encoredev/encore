package bits

import (
	"context"
	"go/token"
	"runtime"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"

	"encr.dev/pkg/github"
	"encr.dev/pkg/paths"
	"encr.dev/v2/app"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/parser"
)

// Extract downloads and extracts a bit into a given directory.
func Extract(ctx context.Context, b *Bit, dst string) error {
	tree, err := github.ParseTree(ctx, b.GitHubTree)
	if err != nil {
		return err
	}
	return github.ExtractTree(ctx, tree, dst)
}

// Describe describes the contents of the bit extracted in dir.
func Describe(ctx context.Context, dir string) (desc *app.Desc, err error) {
	fs := token.NewFileSet()
	errs := perr.NewList(ctx, fs)
	pc := &parsectx.Context{
		Ctx: ctx,
		Log: zerolog.Logger{},
		Build: parsectx.BuildInfo{
			GOARCH: runtime.GOARCH,
			GOOS:   runtime.GOOS,
		},
		MainModuleDir: paths.RootedFSPath(dir, "."),
		FS:            fs,
		ParseTests:    false,
		Errs:          errs,
	}

	defer func() {
		if l, ok := perr.CatchBailout(recover()); ok {
			err = errors.Newf("parse failure:\n%s", l.FormatErrors())
		} else if errs.Len() > 0 {
			err = errors.Newf("parse failure:\n%s", errs.FormatErrors())
		}
	}()

	pp := parser.NewParser(pc)
	res := pp.Parse()
	return app.ValidateAndDescribe(pc, res), nil
}
