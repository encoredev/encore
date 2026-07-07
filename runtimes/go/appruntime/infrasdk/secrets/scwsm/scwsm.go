// Package scwsm implements a secrets provider backed by Scaleway Secret Manager.
package scwsm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"encore.dev/appruntime/infrasdk/secrets/provider"
)

const TypeName = "scaleway_secret_manager"

func init() {
	provider.Register(TypeName, New)
}

// Config is the JSON shape of the provider's "config" object.
type Config struct {
	// Region is the Scaleway region the secrets live in. Required.
	Region string `json:"region"`
	// ProjectID is the Scaleway project that owns the secrets. Required.
	ProjectID string `json:"project_id"`
	// AccessKey/SecretKey is an IAM API key with read access to Secret
	// Manager in the project. Required.
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// New constructs the provider from its JSON config.
func New(raw json.RawMessage) (provider.Provider, error) {
	var c Config
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("scwsm: invalid config: %w", err)
		}
	}
	if c.Region == "" {
		return nil, fmt.Errorf("scwsm: region is required")
	}
	if c.ProjectID == "" {
		return nil, fmt.Errorf("scwsm: project_id is required")
	}
	if c.SecretKey == "" {
		return nil, fmt.Errorf("scwsm: secret_key is required")
	}
	return &scwProvider{cfg: c}, nil
}

type scwProvider struct {
	cfg Config
}

// Load accesses a secret version through Scaleway Secret Manager's
// access-by-path endpoint. The Ref.ID is the secret name; when no version
// is supplied, the latest enabled revision is used.
//
// The Secret Manager REST API is called directly to avoid pulling the
// Scaleway SDK into every Encore app.
func (p *scwProvider) Load(ctx context.Context, ref provider.Ref) (string, error) {
	revision := ref.Version.GetOrElse("latest_enabled")

	query := url.Values{}
	query.Set("secret_path", "/")
	query.Set("secret_name", ref.ID)
	query.Set("project_id", p.cfg.ProjectID)
	u := fmt.Sprintf(
		"https://api.scaleway.com/secret-manager/v1beta1/regions/%s/secrets-by-path/versions/%s/access?%s",
		url.PathEscape(p.cfg.Region), url.PathEscape(revision), query.Encode(),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("scwsm: create request: %w", err)
	}
	req.Header.Set("X-Auth-Token", p.cfg.SecretKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("scwsm: access %s: %w", ref.ID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("scwsm: read response for %s: %w", ref.ID, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("scwsm: access %s: unexpected status %d", ref.ID, resp.StatusCode)
	}

	var payload struct {
		// Data is base64-encoded by the API; encoding/json decodes []byte
		// fields from base64 automatically.
		Data []byte `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("scwsm: decode response for %s: %w", ref.ID, err)
	}
	return string(payload.Data), nil
}
