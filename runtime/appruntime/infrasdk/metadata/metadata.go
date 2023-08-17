package metadata

import (
	"fmt"

	"encore.dev/appruntime/exported/config"
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
	EnvName    string
}

func (md *ContainerMetadata) Labels() map[string]string {
	labels := make(map[string]string)
	if md.ServiceID != "" {
		labels["service_id"] = md.ServiceID
	}
	if md.RevisionID != "" {
		labels["revision_id"] = md.RevisionID
	}
	if md.InstanceID != "" {
		labels["instance_id"] = md.InstanceID
	}
	if md.EnvName != "" {
		labels["env_name"] = md.EnvName
	}
	return labels
}

func MapMetadataLabels[T any](md *ContainerMetadata, fn func(k, v string) T) []T {
	var labels []T
	for k, v := range md.Labels() {
		labels = append(labels, fn(k, v))
	}
	return labels
}

func GetContainerMetadata(cfg *config.Runtime) (*ContainerMetadata, error) {
	for _, collector := range collectorRegistry {
		if collector.matches(cfg.EnvCloud) {
			md, err := collector.collect()
			if err != nil {
				return nil, err
			}
			md.EnvName = cfg.EnvName
			return md, nil
		}
	}
	return nil, fmt.Errorf("no metadata collector found for environment cloud '%s'", cfg.EnvCloud)
}
