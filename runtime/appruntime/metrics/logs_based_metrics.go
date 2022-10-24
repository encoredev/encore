package metrics

import "github.com/rs/zerolog"

func logCounter(logger zerolog.Logger, name string, tags ...string) {
	loggerCtx := logger.With().Str("e_metric_name", name)
	loggerCtx = addTags(loggerCtx, tags...)
	logger = loggerCtx.Logger()
	logger.Trace().Send()
}

func logValue(logger zerolog.Logger, name string, observationKey string, observationValue float64, tags ...string) {
	loggerCtx := logger.With().Str("e_metric_name", name).Float64(observationKey, observationValue)
	loggerCtx = addTags(loggerCtx, tags...)
	logger = loggerCtx.Logger()
	logger.Trace().Send()
}

func addTags(loggerCtx zerolog.Context, tags ...string) zerolog.Context {
	for i := 0; i < len(tags); i += 2 {
		if i+1 < len(tags) {
			loggerCtx = loggerCtx.Str(tags[i], tags[i+1])
		}
	}
	return loggerCtx
}
