//go:build !encore_local

package logging

import (
	"time"

	"github.com/rs/zerolog"
)

// configureZerologOutput configures the zerolog logger's output format.
func configureZerologOutput() {
	// Settings to match what Cloud Logging expects.
	// TODO(andre): change this to vary by cloud provider?
	zerolog.LevelFieldName = "severity"
	zerolog.TimestampFieldName = "timestamp"
	zerolog.TimeFieldFormat = time.RFC3339Nano
}
