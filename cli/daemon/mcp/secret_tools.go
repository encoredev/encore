package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
)

func (m *Manager) registerSecretTools() {
	m.server.AddTool(mcp.NewTool("get_secrets",
		mcp.WithDescription("Retrieve metadata about all secrets used in the currently open Encore, including their usage patterns, which services depend on them, and their configuration. This tool helps understand the application's security requirements and secret management strategy."),
	), m.getSecrets)
}

func (m *Manager) getSecrets(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	md, err := inst.CachedMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Build a map of all secrets and the services that use them
	secretUsageMap := make(map[string][]map[string]interface{})

	// First go through all packages to find secrets
	for _, pkg := range md.Pkgs {
		if len(pkg.Secrets) > 0 && pkg.ServiceName != "" {
			// For each secret in this package
			for _, secretName := range pkg.Secrets {
				// Create usage info
				usageInfo := map[string]interface{}{
					"service_name": pkg.ServiceName,
					"package_path": pkg.RelPath,
				}

				// Add to the map
				if _, exists := secretUsageMap[secretName]; !exists {
					secretUsageMap[secretName] = make([]map[string]interface{}, 0)
				}
				secretUsageMap[secretName] = append(secretUsageMap[secretName], usageInfo)
			}
		}
	}

	// Build the result
	secrets := make([]map[string]interface{}, 0)

	// Convert the map to an array
	for secretName, usages := range secretUsageMap {
		secretInfo := map[string]interface{}{
			"name":   secretName,
			"usages": usages,
		}

		// Count unique services
		serviceSet := make(map[string]bool)
		for _, usage := range usages {
			if svcName, ok := usage["service_name"].(string); ok {
				serviceSet[svcName] = true
			}
		}

		secretInfo["service_count"] = len(serviceSet)

		secrets = append(secrets, secretInfo)
	}

	// Sort by name for consistent output
	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i]["name"].(string) < secrets[j]["name"].(string)
	})

	jsonData, err := json.Marshal(secrets)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal secrets information: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
