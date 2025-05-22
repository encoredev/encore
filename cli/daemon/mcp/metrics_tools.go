package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
)

func (m *Manager) registerMetricsTools() {
	m.server.AddTool(mcp.NewTool("get_metrics",
		mcp.WithDescription("Retrieve comprehensive information about all metrics defined in the currently open Encore, including their types, labels, documentation, and usage across services. This tool helps understand the application's observability and monitoring capabilities."),
	), m.getMetrics)
}

func (m *Manager) getMetrics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	md, err := inst.CachedMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Group metrics by service for better organization
	metricsByService := make(map[string][]map[string]interface{})
	globalMetrics := make([]map[string]interface{}, 0)

	// Process all metrics
	for _, metric := range md.Metrics {
		metricInfo := map[string]interface{}{
			"name":       metric.Name,
			"kind":       metric.Kind.String(),
			"value_type": metric.ValueType.String(),
			"doc":        metric.Doc,
		}

		// Add labels if any
		if len(metric.Labels) > 0 {
			labels := make([]map[string]interface{}, 0, len(metric.Labels))
			for _, label := range metric.Labels {
				labelInfo := map[string]interface{}{
					"key":  label.Key,
					"type": label.Type.String(),
					"doc":  label.Doc,
				}
				labels = append(labels, labelInfo)
			}
			metricInfo["labels"] = labels
		}

		// Add to appropriate group (service-specific or global)
		if metric.ServiceName != nil {
			serviceName := *metric.ServiceName
			if _, exists := metricsByService[serviceName]; !exists {
				metricsByService[serviceName] = make([]map[string]interface{}, 0)
			}
			metricsByService[serviceName] = append(metricsByService[serviceName], metricInfo)
		} else {
			globalMetrics = append(globalMetrics, metricInfo)
		}
	}

	// Build the final result
	result := map[string]interface{}{
		"services": make(map[string]interface{}),
		"global":   globalMetrics,
	}

	// Add each service's metrics
	servicesMap := result["services"].(map[string]interface{})
	for serviceName, metrics := range metricsByService {
		// Sort metrics by name within each service
		sort.Slice(metrics, func(i, j int) bool {
			return metrics[i]["name"].(string) < metrics[j]["name"].(string)
		})
		servicesMap[serviceName] = metrics
	}

	// Also sort global metrics
	sort.Slice(globalMetrics, func(i, j int) bool {
		return globalMetrics[i]["name"].(string) < globalMetrics[j]["name"].(string)
	})

	// Add summary counts
	summary := map[string]interface{}{
		"total_metrics":      len(md.Metrics),
		"global_metrics":     len(globalMetrics),
		"service_count":      len(metricsByService),
		"metrics_by_service": make(map[string]int),
		"metrics_by_kind":    make(map[string]int),
		"metrics_by_type":    make(map[string]int),
	}

	// Count metrics by service
	for service, metrics := range metricsByService {
		summary["metrics_by_service"].(map[string]int)[service] = len(metrics)
	}

	// Count metrics by kind and type
	kindCounts := make(map[string]int)
	typeCounts := make(map[string]int)
	for _, metric := range md.Metrics {
		kindStr := metric.Kind.String()
		kindCounts[kindStr] = kindCounts[kindStr] + 1

		typeStr := metric.ValueType.String()
		typeCounts[typeStr] = typeCounts[typeStr] + 1
	}
	summary["metrics_by_kind"] = kindCounts
	summary["metrics_by_type"] = typeCounts

	result["summary"] = summary

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metrics information: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
