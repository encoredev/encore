package runtime

import (
	"os"

	"github.com/rs/zerolog"
)

var rootLogger = createRootLogger()

func createRootLogger() *zerolog.Logger {
	configureZerologOutput()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	return &logger
}

func Logger() *zerolog.Logger {
	if req, _, ok := CurrentRequest(); ok && req != nil && req.Logger != nil {
		return req.Logger
	}
	return rootLogger
}
