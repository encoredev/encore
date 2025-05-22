package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"encr.dev/cli/daemon/engine/trace2"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

func (m *Manager) registerTraceResources() {
	// Register the trace resources
	m.server.AddResourceTemplate(mcp.NewResourceTemplate(
		"trace://{id}",
		"API trace",
		mcp.WithTemplateDescription("Retrieve detailed information about a specific trace, including all spans, timing information, and associated metadata. This resource is useful for deep debugging of individual requests."),
		mcp.WithTemplateMIMEType("application/json"),
	), m.getTraceResource)
}

func (m *Manager) registerTraceTools() {
	// Add tool for listing traces
	m.server.AddTool(mcp.NewTool("get_traces",
		mcp.WithDescription("Retrieve a list of request traces from the application, including their timing, status, and associated metadata. This tool helps understand the flow of requests through the system and diagnose issues."),
		mcp.WithString("service", mcp.Description("Optional service name to filter traces by. Only returns traces that involve the specified service.")),
		mcp.WithString("endpoint", mcp.Description("Optional endpoint name to filter traces by. Only returns traces that involve the specified endpoint.")),
		mcp.WithString("error", mcp.Description("Optional filter for traces with errors. Set to 'true' to see only failed traces, 'false' for successful traces, or omit to see all traces.")),
		mcp.WithString("limit", mcp.Description("Maximum number of traces to return. Helps manage response size when dealing with many traces.")),
		mcp.WithString("start_time", mcp.Description("ISO format timestamp to filter traces created after this time. Useful for focusing on recent activity.")),
		mcp.WithString("end_time", mcp.Description("ISO format timestamp to filter traces created before this time. Useful for focusing on a specific time period.")),
	), m.listTraces)

	// Add tool for getting a single trace with all spans
	m.server.AddTool(mcp.NewTool("get_trace_spans",
		mcp.WithDescription("Retrieve detailed information about one or more traces, including all spans, timing information, and associated metadata. This tool is useful for deep debugging of individual requests."),
		mcp.WithArray("trace_ids",
			mcp.Items(map[string]any{
				"type":        "string",
				"description": "The unique identifiers of the traces to retrieve. These IDs are returned by the get_traces tool.",
			})),
	), m.getTrace)
}

func (m *Manager) listTraces(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	// Build trace query
	query := &trace2.Query{
		AppID: inst.PlatformOrLocalID(),
		Limit: 100, // Default limit
	}

	if service, ok := request.Params.Arguments["service"].(string); ok && service != "" {
		query.Service = service
	}
	if endpoint, ok := request.Params.Arguments["endpoint"].(string); ok && endpoint != "" {
		query.Endpoint = endpoint
	}
	if errorStr, ok := request.Params.Arguments["error"].(string); ok && errorStr != "" {
		if errorStr == "true" {
			isError := true
			query.IsError = &isError
		} else if errorStr == "false" {
			isError := false
			query.IsError = &isError
		}
	}
	if limitStr, ok := request.Params.Arguments["limit"].(string); ok && limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			query.Limit = limit
		}
	}
	if startTime, ok := request.Params.Arguments["start_time"].(string); ok && startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			query.StartTime = t
		}
	}
	if endTime, ok := request.Params.Arguments["end_time"].(string); ok && endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			query.EndTime = t
		}
	}

	// Collect traces
	var traces []*tracepb2.SpanSummary
	err = m.traces.List(ctx, query, func(span *tracepb2.SpanSummary) bool {
		traces = append(traces, span)
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list traces: %w", err)
	}

	// Convert to JSON
	jsonData, err := json.Marshal(traces)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal traces: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (m *Manager) getTrace(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	traceIDs, ok := request.Params.Arguments["trace_ids"].([]interface{})
	if !ok || len(traceIDs) == 0 {
		return nil, fmt.Errorf("trace_ids is required and must be a non-empty array")
	}

	result := make(map[string][]*tracepb2.TraceEvent)

	for _, traceIDVal := range traceIDs {
		traceID, ok := traceIDVal.(string)
		if !ok || traceID == "" {
			continue // Skip invalid IDs
		}

		// Collect all events for the trace
		var events []*tracepb2.TraceEvent
		err = m.traces.Get(ctx, inst.PlatformOrLocalID(), traceID, func(event *tracepb2.TraceEvent) bool {
			events = append(events, event)
			return true
		})
		if err != nil {
			if errors.Is(err, trace2.ErrNotFound) {
				// Just skip not found traces
				continue
			}
			return nil, fmt.Errorf("failed to get trace %s: %w", traceID, err)
		}

		result[traceID] = events
	}

	// Convert to JSON
	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal traces: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (m *Manager) getTraceResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	traceID := strings.TrimPrefix(request.Params.URI, "trace://")

	// Collect all events for the trace
	var events []*tracepb2.TraceEvent
	err = m.traces.Get(ctx, inst.PlatformOrLocalID(), traceID, func(event *tracepb2.TraceEvent) bool {
		events = append(events, event)
		return true
	})
	if err != nil {
		if errors.Is(err, trace2.ErrNotFound) {
			return nil, fmt.Errorf("trace %s not found", traceID)
		}
		return nil, fmt.Errorf("failed to get trace %s: %w", traceID, err)
	}

	// Convert to JSON
	jsonData, err := json.Marshal(events)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal events: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(jsonData),
		},
	}, nil
}
