package tools

import "encoding/json"

// mustJSON marshals v to a RawMessage, panicking on error since schemas are static.
func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// sourceSchema is the shared tabular data-source abstraction used by both tools.
//
// It accepts one of four forms: a file on disk, columns+array rows, object rows,
// or an inline CSV string.
func sourceSchema() map[string]any {
	return map[string]any{
		"description": "Tabular data: a file on disk or rows passed inline. One of four forms.",
		"oneOf": []map[string]any{
			{
				"type":        "object",
				"description": "Path to a csv/tsv/json/jsonl file on disk. Must resolve inside CHART_MCP_ROOT unless ALLOW_ABS_PATHS=true.",
				"properties":  map[string]any{"file": map[string]any{"type": "string", "description": "file path"}},
				"required":    []string{"file"},
			},
			{
				"type":        "object",
				"description": "Column names plus array rows (compact, good for wide data).",
				"properties": map[string]any{
					"columns": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"rows":    map[string]any{"type": "array", "items": map[string]any{"type": "array"}},
				},
				"required": []string{"columns", "rows"},
			},
			{
				"type":        "object",
				"description": "Array of objects keyed by column name (readable, good for narrow data).",
				"properties": map[string]any{
					"rows": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				},
				"required": []string{"rows"},
			},
			{
				"type":        "object",
				"description": "Inline CSV text with a header row.",
				"properties":  map[string]any{"csv": map[string]any{"type": "string", "description": "CSV contents"}},
				"required":    []string{"csv"},
			},
		},
	}
}

// outputSchema is the rendering configuration used by render_chart.
func outputSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Rendering output configuration.",
		"properties": map[string]any{
			"format": map[string]any{"type": "string", "enum": []string{"png", "svg", "jpeg"}, "default": "png"},
			"width":  map[string]any{"type": "integer", "minimum": int64(1)},
			"height": map[string]any{"type": "integer", "minimum": int64(1)},
			"path":   map[string]any{"type": "string", "description": "Optional path to also write the image to disk."},
		},
	}
}
