//go:build !encore_no_azure

package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	encore "encore.dev"
)

// azureIMDSEndpoint is the Azure Instance Metadata Service endpoint.
// https://learn.microsoft.com/en-us/azure/virtual-machines/instance-metadata-service
// Declared as a variable so that tests can override it to point at an httptest server.
var azureIMDSEndpoint = "http://169.254.169.254/metadata/instance?api-version=2021-02-01"

func init() {
	registerCollector(collectorDesc{
		name: "azure",
		matches: func(envCloud string) bool {
			return envCloud == encore.CloudAzure
		},
		collect: func() (*ContainerMetadata, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, azureIMDSEndpoint, nil)
			if err != nil {
				return nil, fmt.Errorf("azure imds: create request: %w", err)
			}
			// The Metadata header is required by the Azure IMDS service.
			req.Header.Set("Metadata", "true")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				// IMDS may be unavailable outside Azure; return empty metadata gracefully.
				return &ContainerMetadata{}, nil
			}
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("azure imds: read response: %w", err)
			}

			var imds struct {
				Compute struct {
					Location          string `json:"location"`
					Name              string `json:"name"`
					ResourceGroupName string `json:"resourceGroupName"`
					SubscriptionID    string `json:"subscriptionId"`
					VMID              string `json:"vmId"`
				} `json:"compute"`
			}
			if err := json.Unmarshal(body, &imds); err != nil {
				return nil, fmt.Errorf("azure imds: unmarshal response: %w", err)
			}

			// Map IMDS fields to ContainerMetadata:
			//   ServiceID  → resource group (closest equivalent to an ECS service boundary)
			//   RevisionID → empty (no direct equivalent on Azure)
			//   InstanceID → last 8 chars of the VM/container unique ID
			instanceID := imds.Compute.VMID
			if len(instanceID) > 8 {
				instanceID = instanceID[len(instanceID)-8:]
			}

			return &ContainerMetadata{
				ServiceID:  imds.Compute.ResourceGroupName,
				RevisionID: "",
				InstanceID: instanceID,
			}, nil
		},
	})
}
