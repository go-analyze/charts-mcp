package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-analyze/charts-mcp/pkg/chart"
)

// schema helpers keep the JSON schema maps terse.
// mustJSON marshals v to a RawMessage, panicking on error since schemas are static.
func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// schema helpers keep the JSON schema maps terse and avoid repeating literals.

// arrayProp builds an array schema entry; empty itemsType leaves items unconstrained.
func arrayProp(itemsType string) map[string]any {
	schema := map[string]any{"type": "array"}
	if itemsType != "" {
		schema["items"] = map[string]any{"type": itemsType}
	}
	return schema
}

// describedArray builds an array schema entry with typed items and a description.
func describedArray(itemsType, description string) map[string]any {
	return map[string]any{"type": "array",
		"items":       map[string]any{"type": itemsType},
		"description": description}
}

func stringProp(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func numberProp(description string) map[string]any {
	return map[string]any{"type": "number", "description": description}
}

func integerProp(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func boolProp(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

// objectSchema builds an object schema with the given properties and required keys.
func objectSchema(description string, properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "description": description}
	if properties != nil {
		schema["properties"] = properties
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// positionProp accepts a pixel number or a keyword string.
func positionProp(description string) map[string]any {
	return map[string]any{
		"description": description + " Accepts a pixel number or a keyword string.",
		"oneOf":       []map[string]any{{"type": "string"}, {"type": "number"}},
	}
}

// oneOfObjectArray accepts a single entry or an array of entries (axis shape).
func oneOfObjectArray(description string, entry map[string]any) map[string]any {
	return map[string]any{
		"description": description,
		"oneOf": []map[string]any{
			entry,
			{"type": "array", "items": entry},
		},
	}
}

// requestProperties assembles the render_chart input schema: every supported
// ECharts option key at the top level plus the two non-ECharts keys (source,
// output). Descriptions carry the renderer's support contract per field.
func requestProperties() map[string]any {
	return map[string]any{
		"source":          sourceSchema(),
		"output":          outputSchema(),
		"theme":           themeProp(),
		"padding":         paddingProp(),
		"width":           dimensionProp("width", defaultChartWidth),
		"height":          dimensionProp("height", defaultChartHeight),
		"backgroundColor": stringProp("Plain color string painted behind the whole chart. Dark themes also flip text colors for readability, so prefer theme: dark over a bare dark background. Gradient colorStops objects are stripped with a warning."),
		"title":           titleSchema(),
		"legend":          legendSchema(),
		"xAxis":           xAxisSchema(),
		"yAxis":           yAxisSchema(),
		"radar":           radarSchema(),
		"series":          seriesProp(),
	}
}

// sourceSchema is the shared tabular data-source abstraction: the top-level
// source feeds every field and column binding, and a per-series source
// overrides it for that one series.
func sourceSchema() map[string]any {
	return map[string]any{
		"description": "Tabular data feeding column bindings: one of four forms. " +
			"At the top level it feeds every series[].field, series[].columns, and axis " +
			"column binding; inside a series it overrides the shared source for that series alone. " +
			"Files suit long series reused across calls; inline forms cover the rest.",
		"oneOf": []map[string]any{
			objectSchema("Path to a csv/tsv/json/jsonl file on disk; relative paths resolve against the working directory. "+
				"csv/tsv need a header row; json may be an array of objects or an object of named arrays; "+
				"jsonl holds one object per line.",
				map[string]any{"file": stringProp("file path")}, "file"),
			objectSchema("Column names plus array rows (compact, good for wide data). "+
				"Example: {columns: [\"month\", \"sales\"], rows: [[\"Jan\", 120], [\"Feb\", 90]]}; "+
				"null, empty, or \"-\" cells render as gaps.",
				map[string]any{
					"columns": arrayProp("string"),
					"rows":    arrayProp("array"),
				}, "columns", "rows"),
			objectSchema("Array of objects keyed by column name (readable, good for narrow data). "+
				"Example: {rows: [{month: \"Jan\", sales: 120}, {month: \"Feb\", sales: 90}]}",
				map[string]any{
					"rows": arrayProp("object"),
				}, "rows"),
			objectSchema("Inline CSV text with a header row. Example: {csv: \"month,sales\\nJan,120\\nFeb,90\\n\"}",
				map[string]any{"csv": stringProp("CSV contents")}, "csv"),
		},
	}
}

// outputSchema is the render destination; dimensions are option keys (width,
// height), so output carries only the required file path.
func outputSchema() map[string]any {
	return objectSchema("Render destination. The extension of path selects the output format; parent "+
		"directories are created as needed and the response reports the resolved absolute path.",
		map[string]any{
			"path": stringProp("Required. File path ending in .png, .jpg, .jpeg, or .svg (case-insensitive); " +
				"the extension selects the render format."),
		}, "path")
}

func themeProp() map[string]any {
	return stringProp("Built-in palette: " + strings.Join(chart.ThemeNames(), ", ") +
		" (\"default\" aliases light). A theme sets background, text, and series colors together; " +
		"unknown names warn and fall back to light.")
}

func paddingProp() map[string]any {
	return describedArray("integer",
		"Padding around the plot in pixels, 1-4 values with CSS shorthand rules: "+
			"[top], [vertical, horizontal], [top, horizontal, bottom], or [top, right, bottom, left].")
}

func dimensionProp(name string, def int) map[string]any {
	return map[string]any{"type": "integer", "minimum": int64(1),
		"description": fmt.Sprintf("Chart %s in pixels (default %d); setting only one of width/height "+
			"leaves the other on its default, reported in warnings[]. Canvases above 50M total pixels "+
			"add a rendering-time advisory to warnings[].", name, def)}
}

func titleSchema() map[string]any {
	return objectSchema("Chart title block.",
		map[string]any{
			"show":         boolProp("Render the title at all."),
			"text":         stringProp("Title text."),
			"subtext":      stringProp("Subtitle text under the title."),
			"left":         positionProp("Title horizontal position."),
			"top":          positionProp("Title vertical position."),
			"textStyle":    textStyleSchema("Title text style."),
			"subtextStyle": textStyleSchema("Subtitle text style."),
		})
}

func legendSchema() map[string]any {
	return objectSchema("Legend block. backgroundColor is not rendered.",
		map[string]any{
			"show": boolProp("Render the legend at all."),
			"data": describedArray("string", "Series names, in display order; overrides auto-derived names."),
			"align": stringProp("Position of the whole legend block within the chart: left, center, or right. " +
				"Not text alignment; the marker always renders left of its label and entries never reorder."),
			"orient":    stringProp("\"vertical\" (case-insensitive) selects a vertical layout."),
			"padding":   paddingProp(),
			"left":      positionProp("Legend horizontal position."),
			"top":       positionProp("Legend vertical position."),
			"textStyle": textStyleSchema("Legend text style."),
		})
}

func xAxisSchema() map[string]any {
	return oneOfObjectArray("X axis configuration; only the first entry renders.", xAxisEntrySchema())
}

func xAxisEntrySchema() map[string]any {
	return objectSchema("X axis entry; only the first entry renders.",
		map[string]any{
			"type": stringProp("Axis type: \"value\" on the first x-axis flips bar/violin series horizontal; time and log count as category (no continuous axis)."),
			"data": describedArray("string", "Inline category labels; a column binding replaces these when both are set."),
			"column": stringProp("Binding: label column of the top-level source; expands into data, replacing any " +
				"inline labels. Axis bindings do not read per-series sources."),
			"splitNumber":   integerProp("Requested label count."),
			"name":          stringProp("Axis title text."),
			"nameTextStyle": nameTextStyleSchema(),
			"axisLabel": objectSchema("Axis label display. formatter is not rendered on x (y-axis only).",
				map[string]any{
					"show":     boolProp("Explicit false hides labels (rendered transparently)."),
					"color":    stringProp("Label text color."),
					"fontSize": integerProp("Label font size in pixels."),
					"rotate":   numberProp("Label rotation in degrees."),
					"margin":   integerProp("Gap between label and axis in pixels."),
				}),
			"axisLine": objectSchema("Axis line. Only lineStyle.color applies on x; show is not rendered.",
				map[string]any{
					"lineStyle": objectSchema("Axis line style.",
						map[string]any{
							"color":   stringProp("Plain color string (gradients stripped with a warning)."),
							"opacity": numberProp("0-1 alpha scale of the line color."),
						}),
				}),
		})
}

func yAxisSchema() map[string]any {
	return oneOfObjectArray("Y axis configuration; every entry renders, paired per series via yAxisIndex.",
		yAxisEntrySchema())
}

func yAxisEntrySchema() map[string]any {
	return objectSchema("Y axis entry; every entry renders, paired per series via yAxisIndex. Value axes "+
		"auto-range to the data extent, not zero; set min (commonly 0) for a fixed baseline.",
		map[string]any{
			"min":  numberProp("Explicit axis minimum."),
			"max":  numberProp("Explicit axis maximum."),
			"type": stringProp("Axis type: \"category\" on the first y-axis flips bar/violin series horizontal."),
			"data": describedArray("string", "Inline category labels for category-style axes; a column binding replaces these when both are set."),
			"column": stringProp("Binding: label column of the top-level source; expands into data, replacing any " +
				"inline labels. Axis bindings do not read per-series sources."),
			"splitNumber":   integerProp("Requested label count."),
			"name":          stringProp("Axis title text."),
			"nameTextStyle": nameTextStyleSchema(),
			"position":      stringProp("left or right; other values fall back to auto-placement."),
			"splitLine": objectSchema("Split (grid) lines.",
				map[string]any{
					"show": boolProp("Render split lines; defaults to true when omitted (ECharts behavior)."),
				}),
			"axisLabel": objectSchema("Axis label display.",
				map[string]any{
					"show":      boolProp("Explicit false hides labels (rendered transparently)."),
					"color":     stringProp("Label text color."),
					"fontSize":  integerProp("Label font size in pixels."),
					"rotate":    numberProp("Label rotation in degrees."),
					"interval":  integerProp("Label skip count."),
					"margin":    integerProp("Gap between label and axis in pixels; the offset sign follows the axis position."),
					"formatter": stringProp("Template string; every {value} token becomes a humanized 2-decimal number. Functions cannot be expressed in JSON."),
				}),
			"axisLine": axisLineSchema("show toggles the spine line; lineStyle.color applies to the stroke and label text."),
		})
}

func axisLineSchema(description string) map[string]any {
	return objectSchema("Axis line. "+description,
		map[string]any{
			"show": boolProp("Axis line visibility."),
			"lineStyle": objectSchema("Axis line style.",
				map[string]any{
					"color":   stringProp("Plain color string (gradients stripped with a warning)."),
					"opacity": numberProp("0-1 alpha scale of the line color."),
				}),
		})
}

func nameTextStyleSchema() map[string]any {
	return objectSchema("Axis title (name) style.",
		map[string]any{
			"color":    stringProp("Title text color."),
			"fontSize": integerProp("Title font size in pixels."),
		})
}

func textStyleSchema(description string) map[string]any {
	return objectSchema(description,
		map[string]any{
			"color":    stringProp("Text color."),
			"fontSize": numberProp("Font size in pixels."),
		})
}

func radarSchema() map[string]any {
	return objectSchema("Radar configuration; required when a series renders radar. xAxis labels do not build indicators.",
		map[string]any{
			"indicator": map[string]any{"type": "array",
				"description": "Required indicator axes, one per spoke with at least three entries; unnamed max/min derive from the data.",
				"items": objectSchema("One indicator axis.",
					map[string]any{
						"name": stringProp("Indicator (axis) name."),
						"max":  numberProp("Axis maximum; derived from the data when unset."),
						"min":  numberProp("Axis minimum; derived from the data when unset."),
					})},
		})
}

func seriesProp() map[string]any {
	return map[string]any{"type": "array", "minItems": 1, "items": seriesItemSchema(),
		"description": "Chart series; every entry needs a type and exactly one data origin: inline data, " +
			"a bound field, or bound candlestick columns. A series with none binds its table's sole " +
			"numeric column implicitly (warned)."}
}

func seriesItemSchema() map[string]any {
	return objectSchema("One series. Exactly one data origin is required: data (inline ECharts shapes), field "+
		"(a numeric source column), or columns (candlestick OHLC roles); a series carrying only name, type, "+
		"or styling keys binds its table's sole numeric column implicitly (warned), erroring only when zero "+
		"or several numeric columns leave the choice ambiguous. Bindings read the top-level source, or this "+
		"series' own source when set.",
		map[string]any{
			"type": seriesTypeProp(),
			"name": stringProp("Display name for the legend; defaults to the field name when binding a column."),
			"data": seriesDataProp(),
			"field": stringProp("Binding: numeric source column. Missing, empty, and \"-\" cells render as gaps. " +
				"For pie/doughnut/funnel the xAxis labels name the slices; for radar the whole column becomes " +
				"one polygon tuple."),
			"columns":    candlestickColumnsProp(),
			"source":     sourceSchema(),
			"radius":     radiusProp(),
			"yAxisIndex": integerProp("Selects among multiple yAxis entries."),
			"stack":      stringProp("Stack group name. A single group name chart-wide, and when any series stacks, all stackable series must."),
			"symbol":     stringProp("Marker shape: circle, rect, roundRect, diamond, none; triangle and unknowns fall back to square. Chart-wide, set on series[0]."),
			"symbolSize": numberProp("Marker size in pixels, plain number only. Chart-wide, set on series[0]."),
			"lineStyle": objectSchema("Line styling.",
				map[string]any{
					"width": numberProp("Stroke width in pixels; chart-wide, set on series[0]. Dash/dot types are not rendered."),
				}),
			"areaStyle": objectSchema("Presence fills the area under the line; chart-wide from series[0] (fills every series).",
				map[string]any{
					"opacity": numberProp("0-1 alpha scale of the fill."),
				}),
			"barWidth": stringProp("Bar width as a percent of the category slot, e.g. \"35%\"; absolute numbers are dropped with a warning. Chart-wide, set on series[0]."),
			"barGap":   stringProp("Gap between bars as a percent, e.g. \"20%\"; absolute numbers are dropped with a warning. Chart-wide, set on series[0]."),
			"smooth": map[string]any{
				"description": "Line smoothing: true selects 0.5 tension, a 0-1 number sets it directly, false disables. Chart-wide, set on series[0].",
				"oneOf":       []map[string]any{{"type": "boolean"}, {"type": "number"}},
			},
			"itemStyle": objectSchema("Series color; dropped on pie/doughnut/radar/funnel, which color per data item.",
				map[string]any{
					"color":   stringProp("Plain color string overriding the theme palette."),
					"opacity": numberProp("0-1 alpha scale of the series color."),
				}),
			"label": objectSchema("Data labels.",
				map[string]any{
					"show":      boolProp("Render labels; forced on for pie/doughnut regardless."),
					"color":     stringProp("Label text color."),
					"position":  stringProp("bottom, inside, insideBottom shift the label below the anchor; all other values keep the default placement above."),
					"formatter": stringProp("Template string: {a} series name, {b} data name, {c} value, {d} percent of the series total. Functions cannot be expressed in JSON."),
					"distance":  integerProp("Distance from the anchor element in pixels (default 5)."),
				}),
			"markPoint": objectSchema("Mark points; type-based marks only, positional xAxis/yAxis marks are dropped with a warning.",
				map[string]any{
					"symbolSize": integerProp("Point marker size in pixels."),
					"data":       describedArray("object", "Mark entries, e.g. {type: \"max\"}, {type: \"min\"}, {type: \"average\"}."),
				}),
			"markLine": objectSchema("Mark lines; type-based marks only, positional xAxis/yAxis marks are dropped with a warning.",
				map[string]any{
					"data": describedArray("object", "Mark entries, e.g. {type: \"max\"}, {type: \"min\"}, {type: \"average\"}."),
				}),
		}, "type")
}

func seriesTypeProp() map[string]any {
	return map[string]any{"type": "string",
		"enum": chart.SeriesTypes(),
		"description": "Chart type. For a horizontal bar or violin chart keep type bar or violin " +
			"and move the category axis to y: set the first xAxis type to \"value\" or the first " +
			"yAxis type to \"category\". horizontalBar and horizontalViolin are always-horizontal " +
			"alternatives that ignore axis types."}
}

// radiusProp accepts a pixel number, percent string, or [inner, outer] pair.
func radiusProp() map[string]any {
	return map[string]any{
		"description": "Circular chart radius: a pixel number or percent string (\"65%\") sets the outer " +
			"radius; an [inner, outer] pair also sets the doughnut center hole. Pie reads the outer value only.",
		"oneOf": []map[string]any{
			{"type": "number"},
			{"type": "string"},
			{"type": "array", "items": map[string]any{
				"oneOf": []map[string]any{{"type": "number"}, {"type": "string"}},
			}},
		},
	}
}

func seriesDataProp() map[string]any {
	return map[string]any{"type": "array",
		"description": "Inline data items, native ECharts shapes: numbers; null or \"-\" gaps; labeled " +
			"tuples like [\"Mon\", 120] keep only the numeric tail (labels belong on xAxis.data); " +
			"{value, name} objects where value may be a number or array. Pie/doughnut/funnel items without " +
			"a name take the matching xAxis label; radar needs one {name, value: tuple} object per polygon, " +
			"tuple length matching radar.indicator. Candlestick tuples are [open, close, lowest, highest] (note " +
			"the low is third); violin tuples are [min, max]. itemStyle inside data items is not rendered, " +
			"style through the series itemStyle."}
}

func candlestickColumnsProp() map[string]any {
	return objectSchema("Binding: candlestick OHLC column roles. Each role names a source column and rows "+
		"assemble into [open, close, lowest, highest] tuples.",
		map[string]any{
			"open":  stringProp("Open column."),
			"high":  stringProp("High column."),
			"low":   stringProp("Low column."),
			"close": stringProp("Close column."),
		}, "open", "high", "low", "close")
}
