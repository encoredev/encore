package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"
)

func (m *Manager) registerCacheTools() {
	m.server.AddTool(mcp.NewTool("get_cache_keyspaces",
		mcp.WithDescription("Retrieve comprehensive information about all cache keyspaces in the currently open Encore, including their configurations, usage patterns, and the services that interact with them. This tool helps understand the application's caching strategy and data access patterns."),
	), m.getCacheKeyspaces)
}

func (m *Manager) getCacheKeyspaces(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	md, err := inst.CachedMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Find keyspace definition locations from trace nodes
	keyspaceDefLocations := make(map[string]map[string]map[string]interface{})

	// Scan through all packages to find trace nodes related to cache keyspaces
	for _, pkg := range md.Pkgs {
		for _, node := range pkg.TraceNodes {
			// Check for cache keyspace definitions
			if node.GetCacheKeyspace() != nil {
				keyspaceDef := node.GetCacheKeyspace()
				clusterName := keyspaceDef.ClusterName
				keyspaceName := keyspaceDef.VarName

				// Initialize maps if needed
				if _, exists := keyspaceDefLocations[clusterName]; !exists {
					keyspaceDefLocations[clusterName] = make(map[string]map[string]interface{})
				}

				if _, exists := keyspaceDefLocations[clusterName][keyspaceName]; !exists {
					keyspaceDefLocations[clusterName][keyspaceName] = map[string]interface{}{
						"filepath":     node.Filepath,
						"line_start":   node.SrcLineStart,
						"line_end":     node.SrcLineEnd,
						"column_start": node.SrcColStart,
						"column_end":   node.SrcColEnd,
						"package_path": keyspaceDef.PkgRelPath,
					}
				}
			}
		}
	}

	// Build the result
	result := make([]map[string]interface{}, 0)

	// Process all cache clusters
	for _, cluster := range md.CacheClusters {
		clusterInfo := map[string]interface{}{
			"name":            cluster.Name,
			"eviction_policy": cluster.EvictionPolicy,
			"doc":             cluster.Doc,
		}

		// Process keyspaces for this cluster
		keyspaces := make([]map[string]interface{}, 0)
		for _, keyspace := range cluster.Keyspaces {
			keyspaceInfo := map[string]interface{}{
				"service": keyspace.Service,
				"doc":     keyspace.Doc,
			}

			// Add key and value type information from protojson
			if keyspace.KeyType != nil {
				keyTypeData, err := protojson.Marshal(keyspace.KeyType)
				if err == nil {
					var keyTypeJson interface{}
					if err := json.Unmarshal(keyTypeData, &keyTypeJson); err == nil {
						keyspaceInfo["key_type"] = keyTypeJson
					}
				}
			}

			if keyspace.ValueType != nil {
				valueTypeData, err := protojson.Marshal(keyspace.ValueType)
				if err == nil {
					var valueTypeJson interface{}
					if err := json.Unmarshal(valueTypeData, &valueTypeJson); err == nil {
						keyspaceInfo["value_type"] = valueTypeJson
					}
				}
			}

			// Add path pattern if available
			if keyspace.PathPattern != nil {
				pathPattern := make([]string, 0)
				for _, segment := range keyspace.PathPattern.Segments {
					pathPattern = append(pathPattern, segment.Value)
				}
				keyspaceInfo["path_pattern"] = strings.Join(pathPattern, "/")
			}

			// Add definition location if available
			// We need to find the keyspace variable name from the definition data
			// This is approximate as we don't have a direct mapping in the metadata
			if locations, ok := keyspaceDefLocations[cluster.Name]; ok {
				for keyspaceName, location := range locations {
					// Try to match by service
					if location["package_path"] != "" && keyspace.Service != "" {
						// If this location is for a keyspace in this service, add it
						if packageService := findServiceNameForPackage(md, location["package_path"].(string)); packageService == keyspace.Service {
							keyspaceInfo["name"] = keyspaceName
							keyspaceInfo["definition"] = map[string]interface{}{
								"filepath":     location["filepath"],
								"line_start":   location["line_start"],
								"line_end":     location["line_end"],
								"column_start": location["column_start"],
								"column_end":   location["column_end"],
							}
							break
						}
					}
				}
			}

			keyspaces = append(keyspaces, keyspaceInfo)
		}

		clusterInfo["keyspaces"] = keyspaces
		result = append(result, clusterInfo)
	}

	// Sort by cluster name for consistent output
	sort.Slice(result, func(i, j int) bool {
		return result[i]["name"].(string) < result[j]["name"].(string)
	})

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cache keyspaces information: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
