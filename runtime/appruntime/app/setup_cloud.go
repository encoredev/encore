//go:build !encore_local

package app

import (
	"net"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

var devMode = false

func Listen() (net.Listener, error) {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	if port == 0 {
		port = 8080
	}
	return net.Listen("tcp", ":"+strconv.Itoa(port))
}

// ConfigureZerologOutput configures the zerolog Logger's output format.
func ConfigureZerologOutput() {
	// Settings to match what Cloud Logging expects.
	// TODO(andre): change this to vary by cloud provider?
	zerolog.LevelFieldName = "severity"
	zerolog.TimestampFieldName = "timestamp"
	zerolog.TimeFieldFormat = time.RFC3339Nano
}
