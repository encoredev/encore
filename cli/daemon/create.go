package daemon

import (
	"context"

	"encr.dev/cli/daemon/apps"
	daemonpb "encr.dev/proto/encore/daemon"
)

// CreateApp adds tracking for a new app
func (s *Server) CreateApp(ctx context.Context, req *daemonpb.CreateAppRequest) (*daemonpb.CreateAppResponse, error) {
	var options []apps.TrackOption
	if req.Tutorial {
		options = append(options, apps.WithTutorial(req.Template))
	}
	app, err := s.apps.Track(req.AppRoot, options...)
	if err != nil {
		return nil, err
	}
	return &daemonpb.CreateAppResponse{AppId: app.PlatformOrLocalID()}, nil
}
