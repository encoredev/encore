package custommetrics

import (
	"strings"

	"github.com/rs/zerolog"

	encore "encore.dev"
)

type Manager interface {
	Counter(name string, tags map[string]string)
}

func NewManager(appSlug, envCloud string, logger zerolog.Logger) Manager {
	metricPrefix := strings.Replace(appSlug, "-", "_", 1)

	var impl Manager
	switch envCloud {
	case encore.CloudAWS:
		impl = &awsMetricsManager{
			metricPrefix: metricPrefix,
			logger:       logger,
		}
	case encore.CloudGCP:
		impl = &gcpMetricsManager{
			metricPrefix: metricPrefix,
			logger:       logger,
		}
	case encore.CloudAzure:
		// Custom metrics are in still in preview, so we won't be using them for now.
	case encore.EncoreCloud:
		impl = &gcpMetricsManager{
			metricPrefix: metricPrefix,
			logger:       logger,
		}
	case encore.CloudLocal:
		// TODO
	default:
		logger.Error().Str("env_cloud", envCloud).Msg("unexpected cloud environment")
	}
	return impl
}
