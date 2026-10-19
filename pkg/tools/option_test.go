package tools

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-analyze/charts-mcp/pkg/chart"
)

func TestSanitizeOptionStripsIntolerantShapes(t *testing.T) {
	t.Parallel()

	object := decodeOption(t, `{
		"series": [
			{"type": "bar", "data": [1, 2], "barWidth": 40,
				"markPoint": {"data": [{"type": "max"}, {"xAxis": "Wed", "yAxis": 2}]}},
			{"type": "bar", "data": [3, 4], "barGap": 8}
		]
	}`)
	warnings, err := sanitizeOption(object)
	require.NoError(t, err)

	joined := joinWarnings(warnings)
	assert.Contains(t, joined, `series[0].barWidth: absolute sizes are ignored, removed`)
	assert.Contains(t, joined, `series[1].barGap: absolute sizes are ignored, removed`)
	assert.Contains(t, joined, "series[0].markPoint.data[1].xAxis is not rendered, removed")
	assert.Contains(t, joined, "series[0].markPoint.data[1].yAxis is not rendered, removed")

	series := chart.ObjectList(object["series"])
	require.Len(t, series, 2)
	assert.Nil(t, series[0]["barWidth"])
	assert.Nil(t, series[1]["barGap"])
	markData := chart.ObjectList(chart.ObjectMap(series[0]["markPoint"])["data"])
	require.Len(t, markData, 2)
	assert.Equal(t, "max", markData[0]["type"])
	assert.NotContains(t, markData[1], "xAxis")
	assert.NotContains(t, markData[1], "yAxis")
}

func TestSanitizeOptionKeepsPercentSizing(t *testing.T) {
	t.Parallel()

	object := decodeOption(t,
		`{"series": [{"type": "bar", "data": [1, 2], "barWidth": "35%", "barGap": "20%"}]}`)
	warnings, err := sanitizeOption(object)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Equal(t, map[string]any{
		"series": []any{map[string]any{
			"type": "bar", "data": []any{1.0, 2.0}, "barWidth": "35%", "barGap": "20%",
		}},
	}, object)
}

func TestSanitizeOptionGradientColorStops(t *testing.T) {
	t.Parallel()

	object := decodeOption(t, `{"series": [{"type": "line", "data": [1],
		"areaStyle": {"color": {"colorStops": [{"offset": 0, "color": "red"}]}}}],
		"backgroundColor": {"gradient": true},
		"yAxis": [{"axisLabel": {"formatter": {"func": true}}}]}`)

	warnings, err := sanitizeOption(object)
	require.NoError(t, err)

	assert.Nil(t, object["backgroundColor"])
	series := chart.ObjectList(object["series"])[0]
	assert.Empty(t, series["areaStyle"]) // gradient color removed
	yAxis := chart.ObjectList(object["yAxis"])[0]
	axisLabel := chart.ObjectMap(yAxis["axisLabel"])
	assert.NotContains(t, axisLabel, "formatter") // non-string formatter removed

	require.Len(t, warnings, 3)
	joined := strings.Join(warnings, "\n")
	assert.Contains(t, joined, "series[0].areaStyle.color")
	assert.Contains(t, joined, "backgroundColor")
	assert.Contains(t, joined, "yAxis[0].axisLabel.formatter")
}

func TestSanitizeOptionStringColorsPass(t *testing.T) {
	t.Parallel()

	raw := `{"backgroundColor": "#123", "series": [{"type": "line",
		"data": [1], "areaStyle": {"color": "blue"}}]}`

	object := decodeOption(t, raw)
	warnings, err := sanitizeOption(object)
	require.NoError(t, err)
	assert.Equal(t, decodeOption(t, raw), object)
	// the gradient-free fill color decodes fine but is documented as parsed-only
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "series[0].areaStyle.color is accepted but not rendered")
}

func TestLintIgnoredTopLevel(t *testing.T) {
	t.Parallel()

	raw := `{"type": "line", "theme": "cyberpunk",
		"series": [{"type": "line", "data": [1]}]}`

	warnings, err := sanitizeOption(decodeOption(t, raw))
	require.NoError(t, err)

	joined := joinWarnings(warnings)
	assert.Contains(t, joined, "type is ignored; the output.path extension selects the format")
	assert.Contains(t, joined, `unknown theme "cyberpunk"; falling back to light; valid themes: `)
	assert.Contains(t, joined, "vivid-dark") // listed among the valid names
}

func TestSanitizeOptionIgnoresUnknownKeys(t *testing.T) {
	t.Parallel()

	// unknown keys pass through untouched; they may decode in the library, and
	// tracking that surface here would drift out of sync
	raw := `{"tooltip": {}, "fontFamily": "roboto",
		"series": [{"type": "line", "data": [1], "gap": 5}],
		"xAxis": {"boundaryGap": false, "data": ["a"]}}`

	object := decodeOption(t, raw)
	warnings, err := sanitizeOption(object)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Equal(t, decodeOption(t, raw), object)
}

func TestLintIgnoredFieldsWarns(t *testing.T) {
	t.Parallel()

	raw := `{
		"title": {"text": "t", "backgroundColor": "#fff"},
		"series": [{"type": "line", "data": [1], "max": 9, "areaStyle": {"color": "blue"},
			"markPoint": {"data": [{"xAxis": 3}]}},
			{"type": "bar", "data": [2], "barCategoryGap": "20%"}],
		"xAxis": [{"axisLabel": {"formatter": "{value}"}, "axisLine": {"show": true,
			"lineStyle": {"width": 3}}}]
	}`

	warnings, err := sanitizeOption(decodeOption(t, raw))
	require.NoError(t, err)
	joined := strings.Join(warnings, "\n")
	for _, fragment := range []string{
		"title.backgroundColor", "series[0].max", "series[0].areaStyle.color",
		"series[0].markPoint.data[0].xAxis", "series[1].barCategoryGap",
		"xAxis[0].axisLabel.formatter", "(y-axis only)", "xAxis[0].axisLine.show",
	} {
		assert.Contains(t, joined, fragment)
	}
}

func TestLintDimensions(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"width set without height; defaulted height to 400"},
		lintDimensions(decodeOption(t, `{"series": [], "width": 640}`)))
	assert.Equal(t, []string{"height set without width; defaulted width to 600"},
		lintDimensions(decodeOption(t, `{"height": 480}`)))
	assert.Empty(t, lintDimensions(decodeOption(t, `{"width": 640, "height": 480}`)))
	assert.Empty(t, lintDimensions(decodeOption(t, `{"series": []}`)))
}

func TestEffectiveDimensions(t *testing.T) {
	t.Parallel()

	width, height := effectiveDimensions(decodeOption(t, `{"width": 1600}`))
	assert.Equal(t, 1600, width)
	assert.Equal(t, defaultChartHeight, height) // unset dimension keeps its default

	width, height = effectiveDimensions(decodeOption(t, `{}`))
	assert.Equal(t, defaultChartWidth, width)
	assert.Equal(t, defaultChartHeight, height)
}

func TestFormatFromPath(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"chart.png":     "png",
		"chart.PNG":     "png",
		"chart.jpeg":    "jpeg",
		"chart.jpg":     "jpeg",
		"chart.JPG":     "jpeg",
		"dir/chart.svg": "svg",
		"chart.webp":    "",
		"chart":         "",
	}
	for path, want := range cases {
		got, err := formatFromPath(path)
		if want == "" {
			require.Error(t, err, path)
			continue
		}
		require.NoError(t, err, path)
		assert.Equal(t, want, got, path)
	}
}

func TestDecodeErrorNamesOptionField(t *testing.T) {
	t.Parallel()

	err := errors.New("json: cannot unmarshal object into Go struct field " +
		"EChartsOption.series.data.value of type float64")
	assert.Equal(t,
		"option field series.data.value: cannot unmarshal object (expected float64)",
		decodeError(err))

	err = errors.New("json: cannot unmarshal string into Go value of type float64")
	assert.Equal(t,
		"cannot unmarshal string (expected float64)",
		decodeError(err))

	err = errors.New("some other failure")
	assert.Equal(t, "some other failure", decodeError(err))
}

func TestLintChartWideSeriesKeys(t *testing.T) {
	t.Parallel()

	raw := `{
		"series": [
			{"type": "line", "data": [1], "smooth": true},
			{"type": "bar", "data": [2], "symbolSize": 9, "areaStyle": {},
				"lineStyle": {"width": 3}},
			{"type": "line", "data": [3], "smooth": null}
		]
	}`

	warnings, err := sanitizeOption(decodeOption(t, raw))
	require.NoError(t, err)

	joined := joinWarnings(warnings)
	assert.Contains(t, joined,
		"series[1].symbolSize is chart-wide styling (first definition wins); set it on series[0]")
	assert.Contains(t, joined,
		"series[1].areaStyle is chart-wide styling (first definition wins); set it on series[0]")
	assert.Contains(t, joined,
		"series[1].lineStyle.width is chart-wide styling (first definition wins); set it on series[0]")
	// series[0] is the documented home and a null value is unset
	assert.NotContains(t, joined, "series[0].")
	assert.NotContains(t, joined, "series[2]")
}

func TestLintContinuousAxisTypeAndFunnelPercent(t *testing.T) {
	t.Parallel()

	raw := `{
		"xAxis": [{"type": "time", "data": ["a"]}],
		"yAxis": {"type": "log"},
		"series": [
			{"type": "funnel", "label": {"formatter": "{b}: {d}%"}, "data": [1]},
			{"type": "pie", "label": {"formatter": "{d}%"}}
		]
	}`

	object := decodeOption(t, raw)
	warnings, err := sanitizeOption(object)
	require.NoError(t, err)

	joined := joinWarnings(warnings)
	assert.Contains(t, joined, `xAxis[0].type "time" counts as category; there is no continuous axis`)
	assert.Contains(t, joined, `yAxis[0].type "log" counts as category; there is no continuous axis`)
	assert.Contains(t, joined,
		"series[0].label.formatter {d} is not computed on funnel, removed; {a}, {b}, and {c} still render")
	// pie computes {d}, so it stays untouched
	assert.NotContains(t, joined, "series[1]")

	series := chart.ObjectList(object["series"])
	assert.Equal(t, "{b}:", chart.ObjectMap(series[0]["label"])["formatter"])
	assert.Equal(t, "{d}%", chart.ObjectMap(series[1]["label"])["formatter"])
}

func TestLintScopeStructures(t *testing.T) {
	t.Parallel()

	t.Run("warns", func(t *testing.T) {
		raw := `{
			"xAxis": [{"data": ["a"]}, {"data": ["b"], "splitNumber": 4}],
			"series": [{"type": "line", "data": [1]}]
		}`

		warnings, err := sanitizeOption(decodeOption(t, raw))
		require.NoError(t, err)
		assert.Contains(t, joinWarnings(warnings),
			"xAxis has 2 entries; only the first renders and the rest are ignored")
	})

	t.Run("out of range axis index errors", func(t *testing.T) {
		// silent fallback would draw every series on one scale
		raw := `{
			"yAxis": [{"name": "n"}],
			"series": [{"type": "line", "data": [1], "yAxisIndex": 3}]
		}`

		_, err := sanitizeOption(decodeOption(t, raw))
		require.ErrorContains(t, err,
			"series[0].yAxisIndex 3 references a missing yAxis entry (1 declared); add the axis entry or drop the index")
	})

	t.Run("clean", func(t *testing.T) {
		raw := `{
			"xAxis": [{"data": ["a"]}],
			"yAxis": [{}, {}],
			"series": [{"type": "line", "data": [1], "yAxisIndex": 1}]
		}`

		object := decodeOption(t, raw)
		warnings, err := sanitizeOption(object)
		require.NoError(t, err)
		assert.Empty(t, warnings)
		assert.Equal(t, decodeOption(t, raw), object)
	})

	t.Run("default axis index", func(t *testing.T) {
		// yAxisIndex 0 without declared axes keeps the implicit default axis
		raw := `{"series": [{"type": "line", "data": [1], "yAxisIndex": 0}]}`

		warnings, err := sanitizeOption(decodeOption(t, raw))
		require.NoError(t, err)
		assert.Empty(t, warnings)
	})
}

func joinWarnings(warnings []string) string {
	return strings.Join(warnings, "\n")
}

// decodeOption decodes raw JSON into a generic option object.
func decodeOption(t *testing.T, raw string) map[string]any {
	t.Helper()

	var object map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &object))
	return object
}
