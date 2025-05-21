package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"
)

func (m *Manager) registerSrcTools() {
	// Add tool handlers
	m.server.AddTool(mcp.NewTool("get_metadata",
		mcp.WithDescription("Retrieve the complete application metadata, including service definitions, database schemas, API endpoints, and other infrastructure components. This tool provides a comprehensive view of the application's architecture and configuration."),
	), m.getMetadata)

	// Add tool handlers
	m.server.AddTool(mcp.NewTool("get_src_files",
		mcp.WithDescription("Retrieve the contents of one or more source files from the application. This tool is useful for examining specific parts of the codebase or understanding implementation details."),
		mcp.WithArray("files", mcp.Items(map[string]any{
			"type":        "string",
			"description": "List of file paths to retrieve, relative to the application root. Each path should point to a valid source file in the project.",
		})),
	), m.getSrcFiles)

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

	return mcp.NewToolResultText(string(data)), nil
}

func (m *Manager) getSrcFiles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	files, ok := request.Params.Arguments["files"].([]any)
	if !ok || len(files) == 0 {
		return nil, fmt.Errorf("no files provided")
	}

	rtn := map[string]string{}
	for _, file := range files {
		fileStr := file.(string)
		content, err := os.ReadFile(filepath.Join(inst.Root(), fileStr))
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		rtn[fileStr] = string(content)
	}

	jsonData, err := json.Marshal(rtn)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
