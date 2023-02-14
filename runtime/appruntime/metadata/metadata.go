package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	gcemetadata "cloud.google.com/go/compute/metadata"

	encore "encore.dev"
	"encore.dev/appruntime/config"
)

func InstanceID(cfg *config.Runtime) (string, error) {
	switch cfg.EnvCloud {
	case encore.CloudAWS:
		return fargateTaskARN()
	case encore.CloudGCP, encore.EncoreCloud:
		return cloudRunInstanceID()
	default:
		panic("unexpected environment cloud")
	}
}

func fargateTaskARN() (string, error) {
	metadataURI, ok := os.LookupEnv("ECS_CONTAINER_METADATA_URI_V4")
	if !ok {
		return "", fmt.Errorf("unable to read container metadata: metadata URI unset")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURI+"/task", nil)
	if err != nil {
		return "", fmt.Errorf("unable to create metadata request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending metadata request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading metadata response: %w", err)
	}

	taskMetadata := &struct {
		TaskARN string `json:"TaskARN"`
	}{}
	err = json.Unmarshal(body, &taskMetadata)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling metadata response body: %w", err)
	}

	return taskMetadata.TaskARN, nil
}

func cloudRunInstanceID() (string, error) {
	return gcemetadata.InstanceID()
}
