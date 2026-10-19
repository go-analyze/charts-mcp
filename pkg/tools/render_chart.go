package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-analyze/charts"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/go-analyze/charts-mcp/pkg/chart"
	"github.com/go-analyze/charts-mcp/pkg/source"
)

const renderChartName = "render_chart"

// Canonical output format names shared by the schema, validation, and rendering.
const (
	formatPNG  = "png"
	formatJPEG = "jpeg"
	formatSVG  = "svg"
)

// Library painter defaults applied per dimension when an option leaves one unset.
const (
	defaultChartWidth  = 600
	defaultChartHeight = 400
)

// newRenderChartTool builds the render_chart tool schema. The request body is
// an ECharts option document; source and output are the only non-ECharts keys.
// Field descriptions carry the renderer's supported-subset contract so agents
// need no external documentation.
func newRenderChartTool() mcp.Tool {
	description := "Render a chart to a file on disk; the output.path extension (.png, .jpg, .jpeg, or " +
		".svg) selects the format and image bytes are never returned inline. The request body is an " +
		"ECharts option (the renderer's supported subset, documented per field) plus two non-ECharts " +
		"keys: source, a tabular dataset feeding column bindings, and output, the render destination. " +
		"The top-level source is one shared table feeding every binding, axis labels included; a " +
		"per-series source swaps in a private table for that series alone and never feeds axis labels. " +
		"Series data arrives inline (series[].data, native ECharts shapes) or bound to source columns " +
		"through series[].field, series[].columns (candlestick OHLC), and xAxis.column / yAxis[].column " +
		"(category labels). Every series must declare exactly one of those origins; a series holding only " +
		"name/type/styling keys silently binds its table's sole numeric column instead, and errors only " +
		"when zero or several numeric columns leave the choice ambiguous. Candlestick and violin never " +
		"bind implicitly: they need inline OHLC or extent tuples, or candlestick columns. " +
		"Validation failures name the option path and echo the received entry as JSON, so compare that echo " +
		"against the request and resend a corrected shape rather than an identical payload. All series are " +
		"validated and identical failures collapse into one message naming the repeated paths. Charts render through the top-level " +
		"keys exactly as documented; chart-wide styling fields (symbol, symbolSize, lineStyle.width, " +
		"areaStyle, barWidth, barGap, smooth) style every series and are set on series[0], there are no " +
		"top-level equivalents."
	schema := map[string]any{
		"type":        "object",
		"description": description,
		"properties":  requestProperties(),
		"required":    []string{"output", "series"},
	}
	return mcp.NewToolWithRawSchema(renderChartName, description, mustJSON(schema))
}

// outputOpts mirrors the output schema; dimensions are option keys, so the
// destination carries only the required path.
type outputOpts struct {
	Path string `json:"path"`
}

// renderArgs mirrors the render_chart request: an ECharts option plus the two
// non-ECharts keys. Option holds every other key so unknown ones reach the
// lints instead of being dropped by a typed decode.
type renderArgs struct {
	Source any
	Output outputOpts
	Option json.RawMessage
}

// UnmarshalJSON splits the request into source, output, and the option document.
func (a *renderArgs) UnmarshalJSON(data []byte) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return errors.New("arguments must be a JSON object")
	}
	if raw, ok := object["source"]; ok {
		delete(object, "source")
		if string(raw) != "null" {
			if err := json.Unmarshal(raw, &a.Source); err != nil {
				return fmt.Errorf("source: %w", err)
			}
		}
	}
	if raw, ok := object["output"]; ok {
		delete(object, "output")
		if string(raw) != "null" {
			if err := json.Unmarshal(raw, &a.Output); err != nil {
				return fmt.Errorf("output: %w", err)
			}
		}
	}
	option, err := json.Marshal(object)
	if err != nil {
		return err
	}
	a.Option = option
	return nil
}

// renderResult is the structuredContent contract of render_chart.
type renderResult struct {
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	Warnings []string `json:"warnings"`
	Format   string   `json:"format"`
	Bytes    int      `json:"bytes,omitempty"`
	Path     string   `json:"path"`
	Theme    string   `json:"theme"`
	Series   int      `json:"series"`
}

// handleRenderChart renders the option document and writes it to the required
// output.path, returning render stats without inlining any image data.
func handleRenderChart(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args renderArgs
	if err := req.BindArguments(&args); err != nil {
		return toolError("invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Output.Path) == "" {
		return toolError("output.path is required; rendered charts are never returned inline")
	}
	format, err := formatFromPath(args.Output.Path)
	if err != nil {
		return toolError("%s", err.Error())
	}

	optionJSON, warnings, err := args.buildOption()
	if err != nil {
		return toolError("%s", err.Error())
	}

	image, err := renderImage(optionJSON, format)
	if err != nil {
		return toolError("%s", err.Error())
	}

	result := renderResult{
		Warnings: warnings,
		Format:   format,
		Bytes:    len(image),
	}
	if result.Warnings == nil {
		result.Warnings = []string{} // always an array, never null
	}
	result.Width, result.Height = effectiveDimensions(optionJSON)
	result.Theme = resolvedTheme(optionJSON)
	result.Series = countSeries(optionJSON)
	if note := largeCanvasWarning(result.Width, result.Height); note != "" {
		result.Warnings = append(result.Warnings, note)
	}

	written, err := writeChartFile(args.Output.Path, image)
	if err != nil {
		return toolError("%s", err.Error())
	}
	result.Path = written

	text := fmt.Sprintf("rendered %s chart (%s theme) at %dx%d px (%.1f KB)",
		result.Format, result.Theme, result.Width, result.Height, float64(result.Bytes)/1024)
	text += "\nwritten to " + result.Path
	return structuredResult(text, result)
}

// buildOption produces the finalized option JSON for rendering plus any
// warnings: source loading, binding expansion, then the sanitize pass.
func (a renderArgs) buildOption() ([]byte, []string, error) {
	if isEmptyOption(a.Option) {
		return nil, nil, errors.New(
			"chart option is empty; pass an ECharts option with at least one series beside source and output")
	}
	var shared *source.Table
	if a.Source != nil {
		loaded, err := source.Load(a.Source)
		if err != nil {
			return nil, nil, err
		}
		shared = loaded
	}
	// per-series sources give each series its own table; the handler performs
	// all loading so expansion stays pure. Sources arrive in document order,
	// children included.
	tables := make([]*source.Table, 0)
	for _, seriesSource := range chart.SeriesSources(a.Option) {
		if seriesSource.Value == nil {
			tables = append(tables, nil)
			continue
		}
		loaded, err := source.Load(seriesSource.Value)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", seriesSource.Path, err)
		}
		tables = append(tables, loaded)
	}

	expanded, warnings, err := chart.Expand(a.Option, shared, tables)
	if err != nil {
		return nil, nil, err
	}
	sanitized, sanitizeWarnings, err := sanitizeOption(expanded)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, sanitizeWarnings...)
	warnings = append(warnings, lintDimensions(sanitized)...)
	return sanitized, warnings, nil
}

// isEmptyOption reports whether the stripped option document carries no fields,
// the shape of a request whose option was lost or emptied in transit.
func isEmptyOption(option json.RawMessage) bool {
	return len(option) == 0 || string(option) == "{}" || string(option) == "null"
}

// formatFromPath maps the output path extension onto the canonical render format;
// the extension is required, there is no separate format parameter.
func formatFromPath(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return formatPNG, nil
	case ".jpg", ".jpeg":
		return formatJPEG, nil
	case ".svg":
		return formatSVG, nil
	default:
		return "", fmt.Errorf("unsupported output.path extension %q; use .png, .jpg, .jpeg, or .svg",
			filepath.Ext(path))
	}
}

// largeCanvasPixels is the threshold beyond which renders get slow enough that
// a note helps tell an expected long render from a hung one.
const largeCanvasPixels = 50 * 1000 * 1000

// largeCanvasWarning returns a heads-up for very large canvases, empty otherwise.
func largeCanvasWarning(width, height int) string {
	if width <= 0 || height <= 0 || width*height <= largeCanvasPixels {
		return ""
	}
	return fmt.Sprintf("large canvas %dx%d px; rendering may take a while", width, height)
}

// renderImage renders finalized option JSON through the library entry point and
// returns the encoded bytes. Undecodable field shapes surface the offending
// option field path instead of a raw Go type error.
func renderImage(optionJSON []byte, format string) ([]byte, error) {
	var data []byte
	var err error
	switch format {
	case formatPNG:
		data, err = charts.RenderEChartsToPNG(string(optionJSON))
	case formatJPEG:
		data, err = charts.RenderEChartsToJPG(string(optionJSON))
	case formatSVG:
		data, err = charts.RenderEChartsToSVG(string(optionJSON))
	default:
		return nil, fmt.Errorf("unsupported render format %q; use png, jpeg, or svg", format)
	}
	if err != nil {
		return nil, errors.New(decodeError(err))
	}
	return data, nil
}

// decodeError rewrites Go unmarshal errors naming the offending option field,
// e.g. "json: cannot unmarshal object into Go struct field
// EChartsOption.series.data.value of type float64" becomes "option field
// series.data.value: cannot unmarshal object (expected a float64 number)".
// Other errors pass through unchanged.
func decodeError(err error) string {
	rest, ok := strings.CutPrefix(err.Error(), "json: cannot unmarshal ")
	if !ok {
		return err.Error()
	}
	var got, path, want string
	if before, after, found := strings.Cut(rest, " into Go struct field "); found {
		got = before
		if field, typePart, splits := strings.Cut(after, " of type "); splits {
			path = field
			want = typePart
		}
	} else if before, after, found := strings.Cut(rest, " into Go value of type "); found {
		got = before
		want = after
	} else {
		return err.Error()
	}
	if path != "" {
		if dot := strings.Index(path, "."); dot >= 0 { // drop the Go struct name prefix
			path = path[dot+1:]
		}
		if want != "" {
			return fmt.Sprintf("option field %s: cannot unmarshal %s (expected %s)", path, got, want)
		}
		return fmt.Sprintf("option field %s: cannot unmarshal %s", path, got)
	}
	return fmt.Sprintf("cannot unmarshal %s (expected %s)", got, want)
}

// resolvedTheme reports the palette a render used: the named theme when known,
// light for unset or unknown names.
func resolvedTheme(optionJSON []byte) string {
	var parsed struct {
		Theme string `json:"theme"`
	}
	if err := json.Unmarshal(optionJSON, &parsed); err != nil {
		return charts.ThemeLight
	}
	if parsed.Theme != "" && chart.KnownTheme(parsed.Theme) {
		return parsed.Theme
	}
	return charts.ThemeLight
}

// countSeries reports the number of series entries the option renders.
func countSeries(optionJSON []byte) int {
	var parsed struct {
		Series json.RawMessage `json:"series"`
	}
	if err := json.Unmarshal(optionJSON, &parsed); err != nil || len(parsed.Series) == 0 {
		return 0
	}
	var list []map[string]any
	if err := json.Unmarshal(parsed.Series, &list); err == nil {
		return len(list)
	}
	var single map[string]any
	if err := json.Unmarshal(parsed.Series, &single); err == nil && single != nil {
		return 1
	}
	return 0
}

// writeChartFile writes rendered bytes to the output path, creating parent
// directories, and returns the resolved absolute location.
func writeChartFile(path string, data []byte) (string, error) {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if dir := filepath.Dir(resolved); dir != "" {
		if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil {
			return "", fmt.Errorf("create output directory: %w", mkErr)
		}
	}
	if writeErr := os.WriteFile(resolved, data, 0o644); writeErr != nil { //nolint:gosec // unrestricted write paths are intended
		return "", fmt.Errorf("write output file: %w", writeErr)
	}
	return resolved, nil
}
