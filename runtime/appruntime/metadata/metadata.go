package metadata

import (
	"fmt"

	"encore.dev/appruntime/config"
)

type collectorDesc struct {
	name    string
	matches func(envCloud string) bool
	collect func() (*ContainerMetadata, error)
}

var collectorRegistry []collectorDesc

func registerCollector(desc collectorDesc) {
	collectorRegistry = append(collectorRegistry, desc)
}

type ContainerMetadata struct {
	ServiceID  string
	RevisionID string
	InstanceID string
}

func GetContainerMetadata(cfg *config.Runtime) (*ContainerMetadata, error) {
	for _, collector := range collectorRegistry {
		if collector.matches(cfg.EnvCloud) {
			return collector.collect()
		}
	}
	return nil, fmt.Errorf("no metadata collector found for environment cloud '%s'", cfg.EnvCloud)
}
