package logging

import (
	"os"

	"github.com/rs/zerolog"
)

var RootLogger *zerolog.Logger

func init() {
	configureZerologOutput()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	RootLogger = &logger
}
