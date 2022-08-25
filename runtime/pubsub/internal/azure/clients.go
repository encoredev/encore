package azure

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	"encore.dev/appruntime/config"
)

// getClient returns a singleton azure servicebus client for the given project or panics if it cannot be created.
func (mgr *Manager) getClient(cfg *config.AzureServiceBusProvider) *azservicebus.Client {
	mgr.clientMu.RLock()
	client, ok := mgr._clients[cfg.Namespace]
	mgr.clientMu.RUnlock()
	if ok {
		return client
	}
	mgr.clientMu.Lock()
	defer mgr.clientMu.Unlock()
	// Create a new client
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create azure credential: %s", err))
	}
	azclient, err := azservicebus.NewClient(
		fmt.Sprintf("%s.servicebus.windows.net",
			cfg.Namespace), credential, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create azure client: %s", err))
	}
	mgr._clients[cfg.Namespace] = azclient

	return azclient
}
