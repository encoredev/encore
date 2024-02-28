package daemon

import (
	"bytes"
	"context"
	"runtime"

	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/vcs"
	daemonpb "encr.dev/proto/encore/daemon"
)

func (s *Server) DumpMeta(ctx context.Context, req *daemonpb.DumpMetaRequest) (*daemonpb.DumpMetaResponse, error) {
	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	expSet, err := app.Experiments(req.Environ)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// TODO: We should check that all secret keys are defined as well.

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

	bld := builderimpl.Resolve(expSet)
	parse, err := bld.Parse(ctx, builder.ParseParams{
		Build:       buildInfo,
		App:         app,
		Experiments: expSet,
		WorkingDir:  req.WorkingDir,
		ParseTests:  req.ParseTests,
	})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var out []byte
	switch req.Format {
	case daemonpb.DumpMetaRequest_FORMAT_PROTO:
		out, err = proto.Marshal(parse.Meta)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	case daemonpb.DumpMetaRequest_FORMAT_JSON:
		var buf bytes.Buffer
		m := &jsonpb.Marshaler{OrigName: true, EmitDefaults: true, Indent: "  "}
		if err := m.Marshal(&buf, parse.Meta); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		out = buf.Bytes()
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid format")
	}

	return &daemonpb.DumpMetaResponse{Meta: out}, nil
}
