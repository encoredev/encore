package logging

import (
	"log"

	"github.com/rs/zerolog"
)

type zeroLogWriter struct {
	logger zerolog.Logger
	level  zerolog.Level
}

func (z *zeroLogWriter) Write(p []byte) (n int, err error) {
	z.logger.WithLevel(z.level).CallerSkipFrame(3).Msg(string(p))
	return len(p), nil
}

// NewZeroLogAdapter returns a new log.Logger that writes to the given zerolog.Logger at the given level.
func NewZeroLogAdapter(logger zerolog.Logger, level zerolog.Level) *log.Logger {
	zlw := &zeroLogWriter{logger, level}
	return log.New(zlw, "", 0)
}
