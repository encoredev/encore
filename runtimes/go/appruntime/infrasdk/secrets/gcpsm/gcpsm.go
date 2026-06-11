// Package gcpsm implements a secrets provider backed by Google Cloud Secret Manager.
package gcpsm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/option"

	"encore.dev/appruntime/infrasdk/secrets/provider"
)

const TypeName = "gcp_secret_manager"

func init() {
	provider.Register(TypeName, New)
}

// Config is the JSON shape of the provider's "config" object.
type Config struct {
	// ProjectID is the GCP project that owns the secrets. Required.
	ProjectID string `json:"project_id"`
}

// New constructs the provider from its JSON config.
func New(raw json.RawMessage) (provider.Provider, error) {
	var c Config
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("gcpsm: invalid config: %w", err)
		}
	}
	if c.ProjectID == "" {
		return nil, fmt.Errorf("gcpsm: project_id is required")
	}

	var opts []option.ClientOption

	return &gcpProvider{projectID: c.ProjectID, opts: opts}, nil
}

type gcpProvider struct {
	projectID string
	opts      []option.ClientOption

	initOnce sync.Once
	client   *secretmanager.Client
	initErr  error
}

func (p *gcpProvider) Load(ctx context.Context, ref provider.Ref) (string, error) {
	if err := p.ensureClient(ctx); err != nil {
		return "", err
	}
	name := p.resourceName(ref)
	resp, err := p.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{Name: name})
	if err != nil {
		return "", fmt.Errorf("gcpsm: access %s: %w", name, err)
	}
	return string(resp.Payload.Data), nil
}

func (p *gcpProvider) ensureClient(ctx context.Context) error {
	p.initOnce.Do(func() {
		p.client, p.initErr = secretmanager.NewClient(ctx, p.opts...)
	})
	return p.initErr
}

// resourceName builds the secret-version name to access. The Ref.ID may be
// either a bare secret name or a fully-qualified "projects/.../secrets/..."
// resource path. When no version is supplied, "latest" is used.
func (p *gcpProvider) resourceName(ref provider.Ref) string {
	version := ref.Version.GetOrElse("latest")
	if strings.HasPrefix(ref.ID, "projects/") {
		if !strings.Contains(ref.ID, "/versions/") {
			return ref.ID + "/versions/" + version
		}
		return ref.ID
	}
	return fmt.Sprintf("projects/%s/secrets/%s/versions/%s", p.projectID, ref.ID, version)
}
