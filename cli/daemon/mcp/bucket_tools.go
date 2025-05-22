package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"

	"encr.dev/pkg/emulators/storage/gcsemu"
)

func (m *Manager) registerBucketTools() {
	m.server.AddTool(mcp.NewTool("get_storage_buckets",
		mcp.WithDescription("Retrieve comprehensive information about all storage buckets in the currently open Encore, including their configurations, access patterns, and the services that interact with them. This tool helps understand the application's storage architecture and data management strategy."),
	), m.getStorageBuckets)

	m.server.AddTool(mcp.NewTool("get_objects",
		mcp.WithDescription("List and retrieve metadata about objects stored in one or more storage buckets. This tool helps inspect the contents of storage buckets and understand the data stored in them."),
		mcp.WithArray("buckets",
			mcp.Items(map[string]any{
				"type":        "string",
				"description": "List of bucket names to list objects from. Each bucket must be defined in the currently open Encore's storage configuration.",
			})),
	), m.listObjects)
}

func (m *Manager) listObjects(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	app, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}
	clusterNS, err := m.ns.GetActive(ctx, app)
	if err != nil {
		return nil, fmt.Errorf("failed to get active namespace: %w", err)
	}
	dir, err := m.objects.BaseDir(clusterNS.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get base directory: %w", err)
	}
	store := gcsemu.NewFileStore(dir)
	buckets, ok := request.Params.Arguments["buckets"].([]any)
	if !ok {
		return nil, fmt.Errorf("buckets is not an array")
	}
	objects := map[string][]map[string]interface{}{}
	for _, bucket := range buckets {
		bucketName := bucket.(string)
		var bucketObjects []map[string]interface{}
		err = store.Walk(ctx, bucketName, func(ctx context.Context, filename string, fInfo os.FileInfo) error {
			objectInfo := map[string]interface{}{
				"name":          filename,
				"size":          fInfo.Size(),
				"last_modified": fInfo.ModTime(),
				"is_directory":  fInfo.IsDir(),
			}
			bucketObjects = append(bucketObjects, objectInfo)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk bucket objects: %w", err)
		}
		objects[bucketName] = bucketObjects
	}
	jsonData, err := json.Marshal(objects)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal object information: %w", err)
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}

func (m *Manager) getStorageBuckets(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	md, err := inst.CachedMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Build map of services that use each bucket with their operations
	bucketUsageByService := make(map[string][]map[string]interface{})

	for _, svc := range md.Svcs {
		for _, bucketUsage := range svc.Buckets {
			bucketName := bucketUsage.Bucket

			// Convert operations to strings
			operations := make([]string, 0, len(bucketUsage.Operations))
			for _, op := range bucketUsage.Operations {
				operations = append(operations, op.String())
			}

			// Create usage info
			usageInfo := map[string]interface{}{
				"service_name": svc.Name,
				"operations":   operations,
			}

			// Add to map
			if _, exists := bucketUsageByService[bucketName]; !exists {
				bucketUsageByService[bucketName] = make([]map[string]interface{}, 0)
			}
			bucketUsageByService[bucketName] = append(bucketUsageByService[bucketName], usageInfo)
		}
	}

	// Collect bucket definition locations from trace nodes
	bucketDefLocations := make(map[string]map[string]interface{})

	// Find bucket definitions in trace nodes if possible
	// Currently no specific bucket definition node type in the TraceNode,
	// so we leave this empty for now. This could be expanded in the future
	// if the metadata provides better tracking.

	// Process all buckets
	buckets := make([]map[string]interface{}, 0)
	for _, bucket := range md.Buckets {
		bucketInfo := map[string]interface{}{
			"name":      bucket.Name,
			"versioned": bucket.Versioned,
			"public":    bucket.Public,
		}

		// Add documentation if available
		if bucket.Doc != nil {
			bucketInfo["doc"] = *bucket.Doc
		}

		// Add location information if available
		if location, exists := bucketDefLocations[bucket.Name]; exists {
			bucketInfo["definition"] = location
		}

		// Add service usage information
		if usages, exists := bucketUsageByService[bucket.Name]; exists {
			bucketInfo["service_usage"] = usages
		} else {
			bucketInfo["service_usage"] = []map[string]interface{}{}
		}

		buckets = append(buckets, bucketInfo)
	}

	jsonData, err := json.Marshal(buckets)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal storage buckets information: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
