//go:build encore_app

package logging

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/cloud"
)

var RootLogger = configure(appconf.Static, appconf.Runtime)

func configure(static *config.Static, runtime *config.Runtime) zerolog.Logger {
	var logOutput io.Writer = os.Stderr
	if static.TestAsExternalBinary {
		// If we're running as a test and as a binary outside of the Encore Daemon, then we want to
		// log the output via a console logger, rather than the underlying JSON logs.
		logOutput = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.Out = logOutput
		})
	}

	reconfigureZerologFormat(runtime)
	return zerolog.New(logOutput).With().Timestamp().Logger()
}

func reconfigureZerologFormat(runtime *config.Runtime) {
	// Note: if updating this function, also update
	// mapCloudFieldNamesToExpected in cli/cmd/encore/logs.go
	// as that reverses this for log streaming
	switch runtime.EnvCloud {
	case cloud.GCP, cloud.Encore:
		zerolog.LevelFieldName = "severity"
		zerolog.TimestampFieldName = "timestamp"
		zerolog.TimeFieldFormat = time.RFC3339Nano
	default:
	}
}
