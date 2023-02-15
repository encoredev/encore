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
	"github.com/rs/zerolog"

	encore "encore.dev"
	"encore.dev/appruntime/config"
)

type ContainerMetadata struct {
	ServiceID  string
	RevisionID string
	InstanceID string
}

func GetContainerMetadata(cfg *config.Runtime, logger zerolog.Logger) (*ContainerMetadata, error) {
	switch cfg.EnvCloud {
	case encore.CloudAWS:
		return fargateTaskMetadata()
	case encore.CloudGCP, encore.EncoreCloud:
		return cloudRunContainerMetadata(logger)
	default:
		panic(fmt.Sprintf("unexpected environment cloud '%s'", cfg.EnvCloud))
	}
}

func fargateTaskMetadata() (*ContainerMetadata, error) {
	metadataURI, ok := os.LookupEnv("ECS_CONTAINER_METADATA_URI_V4")
	if !ok {
		return nil, fmt.Errorf("unable to read container metadata: metadata URI unset")
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
	defer resp.Body.Close()

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
}

func cloudRunContainerMetadata(logger zerolog.Logger) (*ContainerMetadata, error) {
	service, ok := os.LookupEnv("K_SERVICE")
	if !ok {
		logger.Warn().Str("env_var", "K_SERVICE").Msg("env variable unset")
	}

	revision, ok := os.LookupEnv("K_REVISION")
	if !ok {
		logger.Warn().Str("env_var", "K_REVISION").Msg("env variable unset")
	}

	instanceID, err := gcemetadata.InstanceID()
	if err != nil {
		return nil, err
	}

	return &ContainerMetadata{
		ServiceID:  service,
		RevisionID: revision,
		InstanceID: instanceID[len(instanceID)-8:],
	}, nil
}
