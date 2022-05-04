//go:build !encore_local && encore_internal

package runtime

import (
	"net"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

var devMode = false

func listen() (net.Listener, error) {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	if port == 0 {
		port = 8080
	}
	return net.Listen("tcp", ":"+strconv.Itoa(port))
}

// configureZerologOutput configures the zerolog logger's output format.
func configureZerologOutput() {
	// Settings to match what Cloud Logging expects.
	// TODO(andre): change this to vary by cloud provider?
	zerolog.LevelFieldName = "severity"
	zerolog.TimestampFieldName = "timestamp"
	zerolog.TimeFieldFormat = time.RFC3339Nano
}
