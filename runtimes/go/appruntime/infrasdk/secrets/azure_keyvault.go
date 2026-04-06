//go:build !encore_no_azure

package secrets

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"

	"encore.dev/appruntime/exported/config/infra"
)

func init() {
	newAzureKVProvider = func(cfg *infra.AzureKeyVaultSecretsProvider) (remoteSecretsProvider, error) {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("azure key vault: create credential: %w", err)
		}
		client, err := azsecrets.NewClient(cfg.VaultURL, cred, nil)
		if err != nil {
			return nil, fmt.Errorf("azure key vault: create client: %w", err)
		}
		return &azureKVProvider{client: client}, nil
	}
}

// azureKVProvider fetches secrets from Azure Key Vault.
// Encore secret names map directly to Key Vault secret names.
type azureKVProvider struct {
	client *azsecrets.Client
}

// FetchSecret retrieves the latest version of a secret from Azure Key Vault.
func (p *azureKVProvider) FetchSecret(ctx context.Context, name string) (string, error) {
	// Pass an empty version string to retrieve the latest enabled version.
	resp, err := p.client.GetSecret(ctx, name, "", nil)
	if err != nil {
		return "", fmt.Errorf("azure key vault: get secret %q: %w", name, err)
	}
	if resp.Value == nil {
		return "", fmt.Errorf("azure key vault: secret %q returned no value", name)
	}
	return *resp.Value, nil
}
