package custommetrics

import (
	"github.com/rs/zerolog"

	encore "encore.dev"
)

type Manager interface {
	Counter(name string, tags map[string]string)
}

func NewManager(envCloud string, logger zerolog.Logger) Manager {
	var impl Manager
	switch envCloud {
	case encore.CloudAWS:
		impl = &awsMetricsManager{
			logger: logger,
		}
	case encore.CloudGCP:
		impl = &gcpMetricsManager{
			logger: logger,
		}
	case encore.CloudAzure:
		// Custom metrics are in still in preview, so we won't be using them for now.
	case encore.EncoreCloud:
		// TODO
	case encore.CloudLocal:
		// TODO
	default:
		logger.Error().Str("env_cloud", envCloud).Msg("unexpected cloud environment")
	}
	return impl
}
