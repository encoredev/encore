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

type Label struct {
	key, value string
}

type Labels []Label

func (l Labels) AsMap() map[string]string {
	m := make(map[string]string)
	for _, kv := range l {
		m[kv.key] = kv.value
	}
	return m
}

func (l *Labels) AddNonEmpty(key, value string) *Labels {
	if value != "" {
		*l = append(*l, Label{key, value})
	}
	return l
}

type ContainerMetadata struct {
	ServiceID  string
	RevisionID string
	InstanceID string
	EnvName    string
}

func (md *ContainerMetadata) Labels() Labels {
	var labels Labels
	labels.AddNonEmpty("service_id", md.ServiceID)
	labels.AddNonEmpty("revision_id", md.RevisionID)
	labels.AddNonEmpty("instance_id", md.InstanceID)
	labels.AddNonEmpty("env_name", md.EnvName)
	return labels
}

func MapMetadataLabels[T any](md *ContainerMetadata, fn func(k, v string) T) []T {
	var labels []T
	for _, v := range md.Labels() {
		labels = append(labels, fn(v.key, v.value))
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
