package daemon

import (
	"context"
	"runtime"

	"github.com/cockroachdb/errors"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/vcs"
	daemonpb "encr.dev/proto/encore/daemon"
)

// GenWrappers generates Encore wrappers.
func (s *Server) GenWrappers(ctx context.Context, req *daemonpb.GenWrappersRequest) (*daemonpb.GenWrappersResponse, error) {
	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		return nil, errors.Wrap(err, "resolve app")
	}
	if err := s.genUserFacing(ctx, app); err != nil {
		return nil, err
	}
	return &daemonpb.GenWrappersResponse{}, nil
}

// genUserFacing generates user-facing wrappers.
func (s *Server) genUserFacing(ctx context.Context, app *apps.Instance) error {
	expSet, err := app.Experiments(nil)
	if err != nil {
		return errors.Wrap(err, "resolve experiments")
	}

	vcsRevision := vcs.GetRevision(app.Root())
	buildInfo := builder.BuildInfo{
		BuildTags:          builder.LocalBuildTags,
		CgoEnabled:         true,
		StaticLink:         false,
		Debug:              false,
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		KeepOutput:         false,
		Revision:           vcsRevision.Revision,
		UncommittedChanges: vcsRevision.Uncommitted,
	}

	bld := builderimpl.Resolve(app.Lang(), expSet)
	defer fns.CloseIgnore(bld)

	parse, err := bld.Parse(ctx, builder.ParseParams{
		Build:       buildInfo,
		App:         app,
		Experiments: expSet,
		WorkingDir:  ".",
		ParseTests:  false,
	})
	if err != nil {
		return errors.Wrap(err, "parse app")
	}

	if err := app.CacheMetadata(parse.Meta); err != nil {
		return errors.Wrap(err, "cache metadata")
	}

	err = bld.GenUserFacing(ctx, builder.GenUserFacingParams{
		Build: buildInfo,
		App:   app,
		Parse: parse,
	})
	return errors.Wrap(err, "generate wrappers")
}
