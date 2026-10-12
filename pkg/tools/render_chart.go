package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

const renderChartName = "render_chart"

// newRenderChartTool builds the render_chart tool schema.
func newRenderChartTool() mcp.Tool {
	schema := map[string]any{
		"type":        "object",
		"description": "Render a chart from tabular data to png/svg/jpeg, returned inline as an image block.",
		"properties": map[string]any{
			"source": sourceSchema(),
			"chart":  chartSchema(),
			"output": outputSchema(),
		},
	}
	return mcp.NewToolWithRawSchema(renderChartName,
		"Render a chart from tabular data to png/svg/jpeg, returned inline as an image block.",
		mustJSON(schema))
}

// seriesItemSchema describes one series: either a field mapping or raw pre-shaped values.
func seriesItemSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "A single series, mapped from a column (field) or given as raw values.",
		"properties": map[string]any{
			"field":  map[string]any{"type": "string", "description": "Source column name to plot."},
			"name":   map[string]any{"type": "string", "description": "Display name for the series/legend."},
			"values": map[string]any{"type": "array", "items": map[string]any{}, "description": "Pre-shaped raw data values (no source column needed)."},
		},
	}
}

func chartSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Chart specification. Either the semantic fields below, or a single full ECharts option/option_file escape hatch (never both).",
		"properties": map[string]any{
			"type":     chartTypeSchema(),
			"title":    map[string]any{"type": "string"},
			"subtitle": map[string]any{"type": "string"},
			"x":        map[string]any{"type": "string", "description": "Column name for category labels, positioned by array index."},
			"series":   map[string]any{"type": "array", "items": seriesItemSchema()},
			"theme":    map[string]any{"type": "string", "description": "Named palette from the library's theme set."},
			"background": map[string]any{"type": "string",
				"description": "Background color override."},
			// Escape hatch: exactly one of option / option_file, exclusive with
			// the semantic fields above.
			"option":      map[string]any{"type": "object", "description": "Full ECharts option inline."},
			"option_file": map[string]any{"type": "string", "description": "Path to a full ECharts JSON option file."},
		},
	}
}

func chartTypeSchema() map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        []string{"line", "bar", "pie", "radar", "funnel", "candlestick"},
		"description": "Chart type.",
	}
}

// handleRenderChart is a stub; wiring and rendering land with the renderer.
func handleRenderChart(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("render_chart not implemented yet"), nil
}
