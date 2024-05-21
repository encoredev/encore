package daemon

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"encr.dev/cli/internal/telemetry"
	daemonpb "encr.dev/proto/encore/daemon"
)

func (s *Server) Telemetry(ctx context.Context, req *daemonpb.TelemetryConfig) (*emptypb.Empty, error) {
	if telemetry.UpdateConfig(req.AnonId, req.Enabled, req.Debug) {
		err := telemetry.SaveConfig()
		if err != nil {
			return nil, err
		}
	}
	return new(emptypb.Empty), nil
}
