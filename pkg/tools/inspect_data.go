package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

const inspectDataName = "inspect_data"

// newInspectDataTool builds the inspect_data tool schema.
func newInspectDataTool() mcp.Tool {
	schema := map[string]any{
		"type":        "object",
		"description": "Discover the shape of a tabular data source before charting it: columns, inferred types, null counts, row count, and a small sample.",
		"properties": map[string]any{
			"source":  sourceSchema(),
			"options": inspectOptionsSchema(),
		},
		"required": []string{"source"},
	}
	return mcp.NewToolWithRawSchema(inspectDataName,
		"Inspect the shape of tabular data: columns, inferred types, null counts, row count, and a sample preview.",
		mustJSON(schema))
}

func inspectOptionsSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Inspection options. Caps the returned preview to protect the context window.",
		"properties": map[string]any{
			"sample_rows": map[string]any{"type": "integer", "minimum": int64(0), "default": float64(10),
				"description": "Number of sample rows to include in the preview (default 10)."},
		},
	}
}

// handleInspectData is a stub; wiring and data inference land with rendering.
func handleInspectData(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("inspect_data not implemented yet"), nil
}
