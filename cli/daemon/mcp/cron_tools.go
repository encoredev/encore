package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func (m *Manager) registerCronTools() {
	m.server.AddTool(mcp.NewTool("get_cronjobs",
		mcp.WithDescription("Retrieve detailed information about all scheduled cron jobs in the currently open Encore, including their schedules, endpoints they trigger, and execution history. This tool helps understand the application's background task scheduling and automation capabilities."),
	), m.getCronJobs)
}

func (m *Manager) getCronJobs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	md, err := inst.CachedMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Create a map to find service and endpoint locations
	endpointLocations := make(map[string]map[string]map[string]interface{})

	// Scan through all packages to find trace nodes related to RPC definitions
	for _, pkg := range md.Pkgs {
		for _, node := range pkg.TraceNodes {
			// Check for RPC definitions
			if node.GetRpcDef() != nil {
				rpcDef := node.GetRpcDef()
				serviceName := rpcDef.ServiceName
				rpcName := rpcDef.RpcName

				// Initialize maps if needed
				if _, exists := endpointLocations[serviceName]; !exists {
					endpointLocations[serviceName] = make(map[string]map[string]interface{})
				}

				if _, exists := endpointLocations[serviceName][rpcName]; !exists {
					endpointLocations[serviceName][rpcName] = map[string]interface{}{
						"filepath":     node.Filepath,
						"line_start":   node.SrcLineStart,
						"line_end":     node.SrcLineEnd,
						"column_start": node.SrcColStart,
						"column_end":   node.SrcColEnd,
					}
				}
			}
		}
	}

	// Process cron jobs with location information
	cronjobs := make([]map[string]interface{}, 0)
	for _, job := range md.CronJobs {
		jobInfo := map[string]interface{}{
			"id":       job.Id,
			"title":    job.Title,
			"schedule": job.Schedule,
		}

		// Add documentation if available
		if job.Doc != nil {
			jobInfo["doc"] = *job.Doc
		}

		// Add endpoint information
		if job.Endpoint != nil {
			endpoint := map[string]interface{}{
				"package": job.Endpoint.Pkg,
				"name":    job.Endpoint.Name,
			}

			// If we can find the service for this endpoint, add location info
			for _, svc := range md.Svcs {
				for _, rpc := range svc.Rpcs {
					if rpc.Name == job.Endpoint.Name && (svc.RelPath == job.Endpoint.Pkg || svc.Name == findServiceNameForPackage(md, job.Endpoint.Pkg)) {
						endpoint["service_name"] = svc.Name

						// Add location if we found it
						if locations, ok := endpointLocations[svc.Name]; ok {
							if loc, ok := locations[rpc.Name]; ok {
								endpoint["definition"] = loc
							}
						}

						break
					}
				}
			}

			jobInfo["endpoint"] = endpoint
		}

		cronjobs = append(cronjobs, jobInfo)
	}

	jsonData, err := json.Marshal(cronjobs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cron jobs information: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
