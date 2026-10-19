package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lineChartArgs is a minimal bound line chart over inline csv with a temp
// output file; most cases need only tweak option fields from here.
func lineChartArgs(t *testing.T, name string) map[string]any {
	t.Helper()

	return map[string]any{
		"source": map[string]any{"csv": "month,revenue\nJan,120\nFeb,90\nMar,150\n"},
		"theme":  "dark",
		"title":  map[string]any{"text": "Revenue"},
		"xAxis":  map[string]any{"column": "month"},
		"series": []any{
			map[string]any{"type": "line", "field": "revenue", "name": "Revenue"},
		},
		"output": map[string]any{"path": filepath.Join(t.TempDir(), name)},
	}
}

func TestToolSchemaValid(t *testing.T) {
	t.Parallel()

	tool := newRenderChartTool() // mustJSON panics on invalid schemas
	var schema struct {
		Required   []string `json:"required"`
		Properties struct {
			Output struct {
				Properties map[string]json.RawMessage `json:"properties"`
				Required   []string                   `json:"required"`
			} `json:"output"`
			Series struct {
				Items struct {
					Required   []string `json:"required"`
					Properties struct {
						Type struct {
							Enum []string `json:"enum"`
						} `json:"type"`
					} `json:"properties"`
				} `json:"items"`
			} `json:"series"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(tool.RawInputSchema, &schema))

	// the option is the request: series sits at the top level, output holds only
	// the path, and every series item requires its type
	assert.Equal(t, []string{"output", "series"}, schema.Required)
	assert.Equal(t, []string{"path"}, schema.Properties.Output.Required)
	assert.Contains(t, schema.Properties.Output.Properties, "path")
	assert.NotContains(t, schema.Properties.Output.Properties, "format")
	// dimensions are option keys, not output fields
	assert.NotContains(t, schema.Properties.Output.Properties, "width")
	assert.Equal(t, []string{"type"}, schema.Properties.Series.Items.Required)
	assert.Contains(t, schema.Properties.Series.Items.Properties.Type.Enum, "candlestick")
}

func TestRenderBoundLinePNG(t *testing.T) {
	t.Parallel()

	args := lineChartArgs(t, "chart.png")
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	structured, ok := result.StructuredContent.(renderResult)
	require.True(t, ok)
	assert.Equal(t, "png", structured.Format)
	assert.Empty(t, structured.Warnings)
	assert.Equal(t, defaultChartWidth, structured.Width) // dimensions from the option
	assert.False(t, hasImageBlock(result))               // images never inline

	data, err := os.ReadFile(structured.Path)
	require.NoError(t, err)
	// PNG magic header
	assert.Equal(t, "\x89PNG\r\n\x1a\n", string(data[:8]))
}

func TestRenderSVGAndJpeg(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, format, magic string }{
		{"chart.svg", "svg", "<svg"},
		{"chart.jpeg", "jpeg", "\xff\xd8"},
		{"chart.jpg", "jpeg", "\xff\xd8"},
	} {
		args := lineChartArgs(t, tc.name)
		result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
		require.NoError(t, err)
		assert.False(t, result.IsError)

		structured, ok := result.StructuredContent.(renderResult)
		require.True(t, ok)
		assert.Equal(t, tc.format, structured.Format)
		data, readErr := os.ReadFile(structured.Path)
		require.NoError(t, readErr)
		assert.Contains(t, string(data), tc.magic)
	}
}

func TestRenderInlineDataNoSource(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"title": map[string]any{"text": "Inline"},
		"xAxis": map[string]any{"data": []any{"Jan", "Feb", "Mar"}},
		"series": []any{map[string]any{
			"type": "line", "name": "Revenue", "data": []any{120.0, 145.5, 160.0}}},
		"output": map[string]any{"path": filepath.Join(t.TempDir(), "chart.svg")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.NotNil(t, result.StructuredContent)
}

func TestRenderCandlestickColumns(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"source": map[string]any{"csv": "open,high,low,close\n10,15,9,14\n14,16,11,12\n"},
		"xAxis":  map[string]any{"column": "close"},
		"series": []any{map[string]any{
			"type": "candlestick",
			"columns": map[string]any{
				"open": "open", "high": "high", "low": "low", "close": "close"},
		}},
		"output": map[string]any{"path": filepath.Join(t.TempDir(), "candles.svg")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestRenderDoughnutBoundField(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"source": map[string]any{"csv": "month,revenue\nJan,120\nFeb,90\nMar,150\n"},
		"xAxis":  map[string]any{"column": "month"},
		"series": []any{map[string]any{"type": "doughnut", "field": "revenue"}},
		"output": map[string]any{"path": filepath.Join(t.TempDir(), "doughnut.svg")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestRenderRadarBoundFields(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"source": map[string]any{"csv": "team,speed,power,range\nA,5,7,6\nB,4,8,5\n"},
		"radar": map[string]any{"indicator": []any{
			map[string]any{"name": "speed"}, map[string]any{"name": "power"},
			map[string]any{"name": "range"},
		}},
		"series": []any{
			map[string]any{"type": "radar", "field": "speed"},
			map[string]any{"type": "radar", "field": "power"},
		},
		"output": map[string]any{"path": filepath.Join(t.TempDir(), "radar.svg")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestRenderPerSeriesFileSources(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.csv"), []byte("val\n1\n2\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.csv"), []byte("num\n7\n8\n"), 0o600))

	args := map[string]any{
		"xAxis": map[string]any{"data": []any{"Q1", "Q2"}},
		"series": []any{
			map[string]any{"type": "bar", "name": "A",
				"source": map[string]any{"file": filepath.Join(dir, "a.csv")}, "field": "val"},
			map[string]any{"type": "bar", "name": "B", "data": []any{7.0, 8.0}},
		},
		"output": map[string]any{"path": filepath.Join(dir, "out.svg")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.NotNil(t, result.StructuredContent)
}

func TestRenderSeriesSourceLoadErrorNamesSeries(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"series": []any{map[string]any{
			"type": "line", "source": map[string]any{"file": "missing.csv"}, "field": "v"}},
		"output": map[string]any{"path": filepath.Join(t.TempDir(), "chart.svg")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	requireErrorResult(t, result, err, "series[0]:")
}

func TestRenderUnusedSourceWarns(t *testing.T) {
	t.Parallel()

	args := lineChartArgs(t, "chart.svg")
	args["xAxis"] = map[string]any{"data": []any{"Jan", "Feb", "Mar"}}
	args["series"] = []any{map[string]any{"type": "line", "data": []any{1.0, 2.0}}}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	structured, ok := result.StructuredContent.(renderResult)
	require.True(t, ok)
	assert.Contains(t, structured.Warnings, "source ignored; no field or column bindings reference it")
}

func TestRenderUnknownThemeWarns(t *testing.T) {
	t.Parallel()

	args := lineChartArgs(t, "chart.svg")
	args["theme"] = "cyberpunk-neon"
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	structured, ok := result.StructuredContent.(renderResult)
	require.True(t, ok)
	warnings := strings.Join(structured.Warnings, "\n")
	assert.Contains(t, warnings, "unknown theme \"cyberpunk-neon\"")
	assert.Contains(t, warnings, "valid themes:") // the names are listed for self-correction
}

func TestRenderAxisColumnOverInlineDataWarns(t *testing.T) {
	t.Parallel()

	args := lineChartArgs(t, "chart.svg")
	args["xAxis"] = map[string]any{"column": "month", "data": []any{"A", "B", "C"}}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError) // the bound column wins and the render proceeds

	structured, ok := result.StructuredContent.(renderResult)
	require.True(t, ok)
	assert.Contains(t, strings.Join(structured.Warnings, "\n"),
		"xAxis[0] sets both column and data; the bound column replaces the inline labels")
}

func TestRenderTopLevelTypeWarns(t *testing.T) {
	t.Parallel()

	args := lineChartArgs(t, "chart.svg")
	args["type"] = "line" // the ECharts habit this lint catches
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	structured, ok := result.StructuredContent.(renderResult)
	require.True(t, ok)
	assert.Contains(t, strings.Join(structured.Warnings, "\n"),
		"type is ignored; the output.path extension selects the format")
}

func TestRenderSeriesErrorEchoesEntry(t *testing.T) {
	t.Parallel()

	// a series stripped to scalar keys mid-transit is distinguishable from a
	// simply forgotten data origin through the JSON echo
	args := map[string]any{
		"series": []any{map[string]any{"type": "line", "name": "Visits", "yAxisIndex": 0}},
		"output": map[string]any{"path": filepath.Join(t.TempDir(), "line.svg")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	requireErrorResult(t, result, err, `series[0] needs data`)
	text := resultText(t, result)
	assert.Contains(t, text, `(received: {"name":"Visits","type":"line","yAxisIndex":0})`)
	// the note distinguishes a stripped payload from a forgotten origin
	assert.Contains(t, text, "(no top-level source present)")
}

func TestRenderImplicitBindingRenders(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"source": map[string]any{"csv": "month,revenue\nJan,120\nFeb,90\n"},
		"xAxis":  map[string]any{"column": "month"},
		// a series stripped to scalar keys still renders off the sole numeric column
		"series": []any{map[string]any{"type": "line", "name": "Revenue"}},
		"output": map[string]any{"path": filepath.Join(t.TempDir(), "implicit.svg")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	require.False(t, result.IsError)

	structured, ok := result.StructuredContent.(renderResult)
	require.True(t, ok)
	assert.Contains(t, strings.Join(structured.Warnings, "\n"),
		`series[0] implicitly bound column "revenue"; pass inline data or set field to bind explicitly`)
}

func TestRenderChartWideStylingOnSeriesZero(t *testing.T) {
	t.Parallel()

	args := lineChartArgs(t, "chart.svg")
	args["series"] = []any{
		map[string]any{"type": "line", "field": "revenue", "symbolSize": 8,
			"lineStyle": map[string]any{"width": 3}},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestRenderMissingColumnError(t *testing.T) {
	t.Parallel()

	args := lineChartArgs(t, "chart.svg")
	args["series"] = []any{map[string]any{"type": "line", "field": "profit"}}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	requireErrorResult(t, result, err, `column "profit" not found; available columns: month, revenue`)
}

func TestRenderMissingPathErrors(t *testing.T) {
	t.Parallel()

	args := lineChartArgs(t, "chart.svg")
	delete(args["output"].(map[string]any), "path")
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	requireErrorResult(t, result, err, "output.path is required")
	// failures carry the structured error envelope rather than no payload
}

func TestRenderBlankPathErrors(t *testing.T) {
	t.Parallel()

	args := lineChartArgs(t, "chart.svg")
	args["output"] = map[string]any{"path": "   "}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	requireErrorResult(t, result, err, "output.path is required")
}

func TestRenderUnsupportedExtensionError(t *testing.T) {
	t.Parallel()

	args := lineChartArgs(t, "chart.png")
	args["output"] = map[string]any{"path": filepath.Join(t.TempDir(), "chart.webp")}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	requireErrorResult(t, result, err, `unsupported output.path extension ".webp"`)
}

func TestRenderEmptyOptionErrors(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"output": map[string]any{"path": filepath.Join(t.TempDir(), "chart.png")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	requireErrorResult(t, result, err, "chart option is empty")
}

func TestRenderReportsEffectiveDimensions(t *testing.T) {
	t.Parallel()

	// only width set: height keeps its 400 default and warns about the pairing
	args := lineChartArgs(t, "chart.svg")
	args["width"] = 1600
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	structured, ok := result.StructuredContent.(renderResult)
	require.True(t, ok)
	assert.Equal(t, 1600, structured.Width)
	assert.Equal(t, defaultChartHeight, structured.Height)
	assert.Contains(t, strings.Join(structured.Warnings, "\n"),
		"width set without height; defaulted height to 400")
}

func TestRenderAbsoluteOutputPath(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "nested", "chart.png")
	args := lineChartArgs(t, "chart.png")
	args["output"] = map[string]any{"path": target}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	structured, ok := result.StructuredContent.(renderResult)
	require.True(t, ok)
	assert.Equal(t, target, structured.Path)
	assert.Contains(t, resultText(t, result), "written to "+target)
	data, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "\x89PNG\r\n\x1a\n", string(data[:8]))
}

func TestRenderNumericBarWidthWarnsInsteadOfFailing(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"series": []any{map[string]any{"type": "bar", "data": []any{1, 2}, "barWidth": 40}},
		"output": map[string]any{"path": filepath.Join(t.TempDir(), "bars.svg")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, resultText(t, result), `series[0].barWidth: absolute sizes are ignored, removed`)
}

func TestRenderMarkPointStringAxisWarnsInsteadOfFailing(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"series": []any{map[string]any{"type": "line", "data": []any{1, 2, 3},
			"markPoint": map[string]any{"data": []any{map[string]any{"xAxis": "Wed", "yAxis": 2}}}}},
		"output": map[string]any{"path": filepath.Join(t.TempDir(), "line.svg")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, resultText(t, result), "positional marks are unsupported")
}

func TestRenderChildrenRejected(t *testing.T) {
	t.Parallel()

	args := lineChartArgs(t, "child.svg")
	args["children"] = []any{map[string]any{
		"series": []any{map[string]any{"type": "bar", "field": "revenue"}},
	}}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	requireErrorResult(t, result, err, "children is not supported")
}

func TestRenderDoughnutRadius(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"series": []any{map[string]any{
			"type": "doughnut", "radius": []any{"40%", "70%"},
			"data": []any{
				map[string]any{"value": 335.0, "name": "Direct"},
				map[string]any{"value": 310.0, "name": "Email"},
			}}},
		"output": map[string]any{"path": filepath.Join(t.TempDir(), "ring.png")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	require.NoError(t, err)
	require.False(t, result.IsError)

	structured, ok := result.StructuredContent.(renderResult)
	require.True(t, ok)
	assert.Empty(t, structured.Warnings) // radius is a supported series field
}

func TestRenderUnsupportedSeriesTypeErrors(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"series": []any{map[string]any{"type": "heatMap", "data": []any{1, 2}}},
		"output": map[string]any{"path": filepath.Join(t.TempDir(), "heat.png")},
	}
	result, err := handleRenderChart(t.Context(), callRequest(t, renderChartName, args))
	requireErrorResult(t, result, err, `series[0] has unsupported type "heatMap"; valid types:`)
}
