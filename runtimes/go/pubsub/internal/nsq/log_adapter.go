package nsq

import (
	"strings"

	"github.com/rs/zerolog"
)

type LogAdapter struct{ Logger *zerolog.Logger }

func (l *LogAdapter) Output(maxdepth int, s string) error {
	// Attempt to extract the level, start with cutting on ":"
	lvl, logMsg, found := strings.Cut(s, ":")
	if !found || strings.Contains(lvl, " ") {
		// then if that fails or we have a space in that cut, try cutting on the first space
		newLvl, suffix, _ := strings.Cut(lvl, " ")
		lvl = newLvl

		if found {
			logMsg = suffix + ":" + logMsg
		}
	}

	// Attempt to convert the level string to a zerolog level
	logLevel := l.OutputLevel(lvl)
	if logLevel == zerolog.NoLevel {
		// and if that fails, then just log the message
		logMsg = s
	}

	logMsg = strings.TrimSpace(logMsg)
	if logMsg != "" {
		l.Logger.WithLevel(logLevel).Msg(logMsg)
	}

	return nil
}

func (l *LogAdapter) OutputLevel(lvl string) zerolog.Level {
	switch strings.ToLower(lvl) {
	case "debug", "dbg":
		return zerolog.DebugLevel
	case "info", "inf":
		return zerolog.InfoLevel
	case "warn", "wrn":
		return zerolog.WarnLevel
	case "error", "err":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	default:
		return zerolog.NoLevel
	}
}
