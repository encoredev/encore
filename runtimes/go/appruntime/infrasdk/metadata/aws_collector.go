//go:build !encore_no_aws

package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	encore "encore.dev"
)

func init() {
	registerCollector(collectorDesc{
		name: "aws",
		matches: func(envCloud string) bool {
			return envCloud == encore.CloudAWS
		},
		collect: func() (*ContainerMetadata, error) {
			// Encore supports running on both ECS Fargate and EKS.
			// For Fargate, we can get the metadata from the ECS metadata service.
			// For EKS there doesn't appear to be a standard way to get the metadata, so skip it in that case.
			metadataURI, ok := os.LookupEnv("ECS_CONTAINER_METADATA_URI_V4")
			if !ok {
				return &ContainerMetadata{
					ServiceID:  "",
					RevisionID: "",
					InstanceID: "",
				}, nil
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURI+"/task", nil)
			if err != nil {
				return nil, fmt.Errorf("unable to create metadata request: %w", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("error sending metadata request: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("error reading metadata response: %w", err)
			}

			taskMetadata := &struct {
				ServiceName string `json:"ServiceName"`
				Revision    string `json:"Revision"`
				TaskARN     string `json:"TaskARN"`
			}{}
			err = json.Unmarshal(body, &taskMetadata)
			if err != nil {
				return nil, fmt.Errorf("error unmarshaling metadata response body: %w", err)
			}

			return &ContainerMetadata{
				ServiceID:  taskMetadata.ServiceName,
				RevisionID: taskMetadata.Revision,
				InstanceID: taskMetadata.TaskARN[len(taskMetadata.TaskARN)-8:],
			}, nil
		},
	})
}
