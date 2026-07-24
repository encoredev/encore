package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"
)

func (m *Manager) registerSrcTools() {
	// Add tool handlers
	m.server.AddTool(mcp.NewTool("get_metadata",
		mcp.WithDescription("Retrieve the complete application metadata, including service definitions, database schemas, API endpoints, and other infrastructure components. This tool provides a comprehensive view of the application's architecture and configuration."),
	), m.getMetadata)
}

func (m *Manager) getMetadata(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}
	md, err := inst.CachedMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	data, err := protojson.Marshal(md)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return mcp.NewToolResultText(string(data)), nil
}
