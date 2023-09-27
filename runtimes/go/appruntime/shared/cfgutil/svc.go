package cfgutil

import (
	"encore.dev/appruntime/exported/config"
)

// IsHostedService returns true if the given service is hosted in this container
// or false otherwise.
//
// If no service name is passed in, then this function returns true if any service
// code is running in this container
func IsHostedService(runtime *config.Runtime, serviceName string) bool {
	// No runtime configured services or gateways means all services are running here
	if len(runtime.HostedServices) == 0 && len(runtime.Gateways) == 0 {
		return true
	}

	// If we're not hosting a gateway and no service name is passed in
	// then we're checking if we're running any service code
	if serviceName == "" && len(runtime.HostedServices) > 0 {
		return true
	}

	// Otherwise check if we're hosting this
	for _, service := range runtime.HostedServices {
		if service == serviceName {
			return true
		}
	}

	return false
}
