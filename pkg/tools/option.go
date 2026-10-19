package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-analyze/charts-mcp/pkg/chart"
)

// sanitizeOption strips option shapes the renderer cannot decode (gradient
// color objects where only color strings are accepted, non-string axis label
// formatters) and warns about fields that decode but never render, so agents
// get actionable warnings instead of raw unmarshal errors or silent styling
// gaps. Unknown keys pass through untouched; the renderer's accepted surface is
// too fluid to lint against. Malformed option JSON is an error rather than a
// render-time failure.
func sanitizeOption(optionJSON []byte) ([]byte, []string, error) {
	var object map[string]any
	if err := json.Unmarshal(optionJSON, &object); err != nil {
		return nil, nil, fmt.Errorf("parse chart option: %w", err)
	}
	var warnings []string
	axisCount := len(objectList(object["yAxis"]))
	lintIgnoredTopLevel(object, &warnings)
	stripColor("backgroundColor", object, "backgroundColor", &warnings)
	for i, series := range objectList(object["series"]) {
		prefix := fmt.Sprintf("series[%d]", i)
		// a missing axis target silently draws every series on one scale,
		// corrupting dual-axis charts, so it fails instead of warning
		if idx, ok := series["yAxisIndex"].(float64); ok && (idx < 0 || idx >= float64(max(axisCount, 1))) {
			return nil, nil, fmt.Errorf(
				"%s.yAxisIndex %d references a missing yAxis entry (%d declared); add the axis entry or drop the index",
				prefix, int(idx), axisCount)
		}
		if i > 0 {
			lintChartWideSeries(prefix, series, &warnings)
		}
		stripFunnelPercentToken(prefix, series, &warnings)
		if style := stringMap(series["itemStyle"]); style != nil {
			stripColor(prefix+".itemStyle.color", style, "color", &warnings)
		}
		if style := stringMap(series["areaStyle"]); style != nil {
			stripColor(prefix+".areaStyle.color", style, "color", &warnings)
		}
		stripNonPercent(prefix+".barWidth", series, "barWidth", &warnings)
		stripNonPercent(prefix+".barGap", series, "barGap", &warnings)
		if dataHasItemStyle(series["data"]) {
			warnings = append(warnings,
				prefix+".data item styles are not rendered; styling comes from the series itemStyle")
		}
		for _, markKey := range []string{"markPoint", "markLine"} {
			stripMarkPositions(prefix, series, markKey, &warnings)
		}
	}
	for _, axis := range []struct{ key, label string }{{"xAxis", "xAxis"}, {"yAxis", "yAxis"}} {
		for i, entry := range objectList(object[axis.key]) {
			prefix := fmt.Sprintf("%s[%d]", axis.label, i)
			lintContinuousAxisType(prefix, entry, &warnings)
			if labels := stringMap(entry["axisLabel"]); labels != nil {
				stripNonString(prefix+".axisLabel.formatter", labels, "formatter",
					"only template strings are supported; removed", &warnings)
			}
			if line := stringMap(entry["axisLine"]); line != nil {
				if style := stringMap(line["lineStyle"]); style != nil {
					stripColor(prefix+".axisLine.lineStyle.color", style, "color", &warnings)
				}
			}
		}
	}
	lintIgnoredFields(object, &warnings)
	if len(warnings) == 0 {
		return optionJSON, nil, nil
	}
	updated, err := json.Marshal(object)
	if err != nil {
		return optionJSON, warnings, err
	}
	return updated, warnings, nil
}

// lintIgnoredTopLevel warns on known-but-inert top-level keys: type is the
// output-format slot overwritten by the render entry point, and unknown theme
// names fall back to the default palette.
func lintIgnoredTopLevel(object map[string]any, warnings *[]string) {
	if _, ok := object["type"]; ok {
		*warnings = append(*warnings,
			"type is ignored; the output.path extension selects the format and chart types live on series[].type")
	}
	if theme, ok := object["theme"].(string); ok && theme != "" && !chart.KnownTheme(theme) {
		*warnings = append(*warnings, fmt.Sprintf(
			"unknown theme %q; falling back to light; valid themes: %s",
			theme, strings.Join(chart.ThemeNames(), ", ")))
	}
}

// lintIgnoredFields reports documented no-op fields so silent styling gaps
// become visible. The list mirrors the "Not rendered" behavior of the
// renderer's ECharts adapter; extend both together.
func lintIgnoredFields(object map[string]any, warnings *[]string) {
	const advice = "is accepted but not rendered"
	for _, top := range []struct{ key, label string }{{"title", "title"}, {"legend", "legend"}} {
		if component := stringMap(object[top.key]); component != nil {
			if _, ok := component["backgroundColor"]; ok {
				*warnings = append(*warnings, top.label+".backgroundColor "+advice)
			}
		}
	}
	for i, series := range objectList(object["series"]) {
		prefix := fmt.Sprintf("series[%d]", i)
		for _, key := range []string{"max", "min", "barCategoryGap"} {
			if _, ok := series[key]; ok {
				*warnings = append(*warnings, prefix+"."+key+" "+advice)
			}
		}
		if style := stringMap(series["areaStyle"]); style != nil {
			if color, ok := style["color"].(string); ok && color != "" {
				*warnings = append(*warnings,
					prefix+".areaStyle.color "+advice+"; use itemStyle or areaStyle.opacity")
			}
		}
	}
	for i, entry := range objectList(object["xAxis"]) {
		prefix := fmt.Sprintf("xAxis[%d]", i)
		if labels := stringMap(entry["axisLabel"]); labels != nil {
			if formatter, ok := labels["formatter"].(string); ok && formatter != "" {
				*warnings = append(*warnings, prefix+".axisLabel.formatter "+advice+" (y-axis only)")
			}
		}
		if line := stringMap(entry["axisLine"]); line != nil {
			if _, ok := line["show"]; ok {
				*warnings = append(*warnings, prefix+".axisLine.show "+advice)
			}
			lintWidth(prefix+".axisLine.lineStyle.width", stringMap(line["lineStyle"]), advice, warnings)
		}
	}
	for i, entry := range objectList(object["yAxis"]) {
		lintWidth(fmt.Sprintf("yAxis[%d].axisLine.lineStyle.width", i),
			stringMap(stringMap(entry["axisLine"])["lineStyle"]), advice, warnings)
	}
	if entries := objectList(object["xAxis"]); len(entries) > 1 {
		*warnings = append(*warnings, fmt.Sprintf(
			"xAxis has %d entries; only the first renders and the rest are ignored", len(entries)))
	}
}

// lintWidth warns when a lineStyle width is present.
func lintWidth(path string, style map[string]any, advice string, warnings *[]string) {
	if _, ok := style["width"]; ok {
		*warnings = append(*warnings, path+" "+advice)
	}
}

// stripColor removes key from parent when set to a non-string, warning that
// only plain color strings decode.
func stripColor(path string, parent map[string]any, key string, warnings *[]string) {
	stripNonString(path, parent, key, "gradient color objects are not supported, removed; plain color strings only", warnings)
}

// stripNonString removes key from parent when set to a non-string, adding a
// warning that names the field.
func stripNonString(path string, parent map[string]any, key, reason string, warnings *[]string) {
	value, ok := parent[key]
	if !ok || value == nil {
		return
	}
	if _, isString := value.(string); isString {
		return
	}
	delete(parent, key)
	*warnings = append(*warnings, path+": "+reason)
}

// stripNonPercent removes bar sizing keys set to non-percent values; absolute
// pixel numbers have no slot-ratio equivalent and are dropped with a hint.
func stripNonPercent(path string, parent map[string]any, key string, warnings *[]string) {
	value, ok := parent[key]
	if !ok {
		return
	}
	if text, isString := value.(string); isString && strings.HasSuffix(text, "%") {
		return
	}
	delete(parent, key)
	*warnings = append(*warnings, path+
		": absolute sizes are ignored, removed; use a percent string like \"35%\"")
}

// stripMarkPositions deletes positional xAxis/yAxis fields from mark data;
// only type-based marks (max, min, average) render, so dropping them keeps the
// option decodable regardless of the value shape used.
func stripMarkPositions(prefix string, series map[string]any, markKey string, warnings *[]string) {
	mark := stringMap(series[markKey])
	if mark == nil {
		return
	}
	for j, entry := range objectList(mark["data"]) {
		for _, pos := range []string{"xAxis", "yAxis"} {
			if _, ok := entry[pos]; ok {
				delete(entry, pos)
				*warnings = append(*warnings, fmt.Sprintf(
					"%s.%s.data[%d].%s is not rendered, removed; positional marks are unsupported (type-based marks only)",
					prefix, markKey, j, pos))
			}
		}
	}
}

// chartWideSeriesKeys are the series styling fields applied chart-wide from
// their first definition (renderer applyChartFields); lineStyle joins them
// through its width field only.
var chartWideSeriesKeys = []string{"symbol", "symbolSize", "areaStyle", "barWidth", "barGap", "smooth"}

// lintChartWideSeries warns when a non-first series sets chart-wide styling:
// the first definition wins and styles every series, so a later value either
// applies to everything or is dropped, never styles one series.
func lintChartWideSeries(path string, series map[string]any, warnings *[]string) {
	for _, key := range chartWideSeriesKeys {
		if series[key] == nil {
			continue
		}
		*warnings = append(*warnings, fmt.Sprintf(
			"%s.%s is chart-wide styling (first definition wins); set it on series[0]", path, key))
	}
	if _, ok := stringMap(series["lineStyle"])["width"]; ok {
		*warnings = append(*warnings, path+
			".lineStyle.width is chart-wide styling (first definition wins); set it on series[0]")
	}
}

// lintContinuousAxisType warns when an axis declares a time or log type; the
// renderer has no continuous scale and treats both as category.
func lintContinuousAxisType(path string, entry map[string]any, warnings *[]string) {
	axisType, _ := entry["type"].(string)
	if axisType == "time" || axisType == "log" {
		*warnings = append(*warnings, fmt.Sprintf(
			"%s.type %q counts as category; there is no continuous axis", path, axisType))
	}
}

// stripFunnelPercentToken removes the {d} percent token from funnel label
// formatters; the renderer never computes funnel percentages and the raw token
// renders as 0, so dropping it keeps labels honest.
func stripFunnelPercentToken(path string, series map[string]any, warnings *[]string) {
	if seriesType, _ := series["type"].(string); seriesType != "funnel" {
		return
	}
	label := stringMap(series["label"])
	template, ok := label["formatter"].(string)
	if !ok || !strings.Contains(template, "{d}") {
		return
	}
	trimmed := strings.ReplaceAll(template, "{d}%", "")
	trimmed = strings.TrimSpace(strings.ReplaceAll(trimmed, "{d}", ""))
	if trimmed == "" {
		delete(label, "formatter")
	} else {
		label["formatter"] = trimmed
	}
	*warnings = append(*warnings, path+
		".label.formatter {d} is not computed on funnel, removed; {a}, {b}, and {c} still render")
}

// objectList decodes a value that may be one object, an array of objects, or
// absent.
func objectList(value any) []map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return []map[string]any{typed}
	case []any:
		list := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if object, ok := item.(map[string]any); ok {
				list = append(list, object)
			}
		}
		return list
	default:
		return nil
	}
}

// stringMap returns value as an object, or nil when it is anything else.
func stringMap(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

// dataHasItemStyle reports whether any raw data entry carries an itemStyle.
func dataHasItemStyle(value any) bool {
	items, ok := value.([]any)
	if !ok {
		_, ok := stringMap(value)["itemStyle"]
		return ok
	}
	for _, item := range items {
		if object := stringMap(item); object != nil {
			if _, ok := object["itemStyle"]; ok {
				return true
			}
		}
	}
	return false
}

// lintDimensions warns when only one of width/height is set; the unset
// dimension keeps its library default, which is worth surfacing since the two
// are coupled in user intent.
func lintDimensions(optionJSON []byte) []string {
	var object map[string]any
	if err := json.Unmarshal(optionJSON, &object); err != nil {
		return nil
	}
	widthSet, heightSet := positiveNumber(object["width"]), positiveNumber(object["height"])
	var warnings []string
	if widthSet && !heightSet {
		warnings = append(warnings, fmt.Sprintf(
			"width set without height; defaulted height to %d", defaultChartHeight))
	}
	if heightSet && !widthSet {
		warnings = append(warnings, fmt.Sprintf(
			"height set without width; defaulted width to %d", defaultChartWidth))
	}
	return warnings
}

// positiveNumber reports whether value is a JSON number above zero; option
// dimensions arrive as float64 from decoding.
func positiveNumber(value any) bool {
	number, ok := value.(float64)
	return ok && number > 0
}

// effectiveDimensions reports the rendered chart size from the finalized
// option, applying the library's 600x400 defaults for unset dimensions.
func effectiveDimensions(optionJSON []byte) (int, int) {
	var parsed struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	if err := json.Unmarshal(optionJSON, &parsed); err != nil {
		return defaultChartWidth, defaultChartHeight
	}
	width, height := parsed.Width, parsed.Height
	if width <= 0 {
		width = defaultChartWidth
	}
	if height <= 0 {
		height = defaultChartHeight
	}
	return width, height
}
