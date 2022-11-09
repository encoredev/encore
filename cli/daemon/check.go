package daemon

import (
	daemonpb "encr.dev/proto/encore/daemon"
)

// Check checks the app for compilation errors.
func (s *Server) Check(req *daemonpb.CheckRequest, stream daemonpb.Daemon_CheckServer) error {
	slog := &streamLog{stream: stream, buffered: false}
	log := newStreamLogger(slog)
	buildDir, err := s.mgr.Check(stream.Context(), req.AppRoot, req.WorkingDir, req.CodegenDebug)

	exitCode := 0
	if err != nil {
		exitCode = 1
		log.Error().Msg(err.Error())
	}

	if req.CodegenDebug && buildDir != "" {
		log.Info().Msgf("wrote generated code to: %s", buildDir)
	}
	streamExit(stream, exitCode)
	return nil
}
