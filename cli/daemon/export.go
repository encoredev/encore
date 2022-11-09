package daemon

import (
	"go/scanner"

	"encr.dev/cli/daemon/export"
	daemonpb "encr.dev/proto/encore/daemon"
)

// Export exports the app.
func (s *Server) Export(req *daemonpb.ExportRequest, stream daemonpb.Daemon_ExportServer) error {
	slog := &streamLog{stream: stream, buffered: false}
	log := newStreamLogger(slog)

	exitCode := 0
	success, err := export.Docker(stream.Context(), req, log)
	if err != nil {
		exitCode = 1
		if list, ok := err.(scanner.ErrorList); ok {
			for _, e := range list {
				log.Error().Msg(e.Error())
			}
		} else {
			log.Error().Msg(err.Error())
		}
	} else if !success {
		exitCode = 1
	}

	streamExit(stream, exitCode)
	return nil
}
