package metrics

import "github.com/rs/zerolog"

func logCounter(logger zerolog.Logger, name string, tags ...string) {
	loggerCtx := logger.With().Str("e_metric_name", name)
	i := 0
	for i < len(tags) {
		if i+1 < len(tags) {
			loggerCtx = loggerCtx.Str(tags[i], tags[i+1])
		}
		i += 2
	}
	logger = loggerCtx.Logger()
	logger.Trace().Send()
}
