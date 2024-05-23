package daemon

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"encr.dev/cli/internal/telemetry"
	daemonpb "encr.dev/proto/encore/daemon"
)

func (s *Server) Telemetry(ctx context.Context, req *daemonpb.TelemetryConfig) (*emptypb.Empty, error) {
	telemetry.UpdateConfig(req.AnonId, req.Enabled, req.Debug)
	return new(emptypb.Empty), nil
}
