//go:build !encore_no_gcp

package metadata

import (
	"fmt"
	"os"
	"strings"

	gcemetadata "cloud.google.com/go/compute/metadata"

	encore "encore.dev"
)

func init() {
	registerCollector(collectorDesc{
		name: "cloud_run",
		matches: func(envCloud string) bool {
			return envCloud == encore.EncoreCloud || envCloud == encore.CloudGCP
		},
		collect: func() (*ContainerMetadata, error) {
			service, ok := os.LookupEnv("K_SERVICE")
			if !ok {
				return nil, fmt.Errorf("unable to get service ID: env variable '%s' unset", "K_SERVICE")
			}

			revision, ok := os.LookupEnv("K_REVISION")
			if !ok {
				return nil, fmt.Errorf("unable to get revision ID: env variable '%s' unset", "K_REVISION")
			}
			revision = strings.TrimPrefix(revision, service+"-")

			instanceID, err := gcemetadata.InstanceID()
			if err != nil {
				return nil, fmt.Errorf("unable to get instance ID: %w", err)
			}

			return &ContainerMetadata{
				ServiceID:  service,
				RevisionID: revision,
				InstanceID: instanceID[len(instanceID)-8:],
			}, nil
		},
	})
}
