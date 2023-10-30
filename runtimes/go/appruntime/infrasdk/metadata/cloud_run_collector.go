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

			instanceID, ok := os.LookupEnv("K_POD")
			if !ok {
				// If we don't have a K8s POD name, we're running on Cloud Run and can get the instance ID from the metadata server
				var err error
				instanceID, err = gcemetadata.InstanceID()
				if err != nil {
					return nil, fmt.Errorf("unable to get instance ID: %w", err)
				}
				instanceID = instanceID[len(instanceID)-8:]
			} else {
				// If we have a K8s POD name, take the last part of it which is the random pod ID
				// On GKE, the InstanceID appears to be the Node, so if the multiple replicas are running
				// on the same InstanceID then we'd have a collision. This is unlikely, but possible -
				// hence why we use the pod ID instead.
				idx := strings.LastIndex(instanceID, "-")
				if idx == -1 {
					return nil, fmt.Errorf("invalid instance ID '%s'", instanceID)
				}
				instanceID = instanceID[idx+1:]
			}

			return &ContainerMetadata{
				ServiceID:  service,
				RevisionID: revision,
				InstanceID: instanceID,
			}, nil
		},
	})
}
