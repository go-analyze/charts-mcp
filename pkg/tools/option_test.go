package tools

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeOptionStripsIntolerantShapes(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"series": [
			{"type": "bar", "data": [1, 2], "barWidth": 40,
				"markPoint": {"data": [{"type": "max"}, {"xAxis": "Wed", "yAxis": 2}]}},
			{"type": "bar", "data": [3, 4], "barGap": 8}
		]
	}`)
	sanitized, warnings, err := sanitizeOption(raw)
	require.NoError(t, err)

	joined := joinWarnings(warnings)
	assert.Contains(t, joined, `series[0].barWidth: absolute sizes are ignored, removed`)
	assert.Contains(t, joined, `series[1].barGap: absolute sizes are ignored, removed`)
	assert.Contains(t, joined, "series[0].markPoint.data[1].xAxis is not rendered, removed")
	assert.Contains(t, joined, "series[0].markPoint.data[1].yAxis is not rendered, removed")

	var object struct {
		Series []struct {
			BarWidth  any `json:"barWidth"`
			BarGap    any `json:"barGap"`
			MarkPoint struct {
				Data []map[string]any `json:"data"`
			} `json:"markPoint"`
		} `json:"series"`
	}
	require.NoError(t, json.Unmarshal(sanitized, &object))
	require.Len(t, object.Series, 2)
	assert.Nil(t, object.Series[0].BarWidth)
	assert.Nil(t, object.Series[1].BarGap)
	require.Len(t, object.Series[0].MarkPoint.Data, 2)
	assert.Equal(t, "max", object.Series[0].MarkPoint.Data[0]["type"])
	assert.NotContains(t, object.Series[0].MarkPoint.Data[1], "xAxis")
	assert.NotContains(t, object.Series[0].MarkPoint.Data[1], "yAxis")
}

func TestSanitizeOptionKeepsPercentSizing(t *testing.T) {
	t.Parallel()

	sanitized, warnings, err := sanitizeOption([]byte(
		`{"series": [{"type": "bar", "data": [1, 2], "barWidth": "35%", "barGap": "20%"}]}`))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.JSONEq(t, `{"series": [{"type": "bar", "data": [1, 2], "barWidth": "35%", "barGap": "20%"}]}`,
		string(sanitized))
}

func TestSanitizeOptionGradientColorStops(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"series": [{"type": "line", "data": [1],
		"areaStyle": {"color": {"colorStops": [{"offset": 0, "color": "red"}]}}}],
		"backgroundColor": {"gradient": true},
		"yAxis": [{"axisLabel": {"formatter": {"func": true}}}]}`)

	sanitized, warnings, err := sanitizeOption(raw)
	require.NoError(t, err)

	var object map[string]any
	require.NoError(t, json.Unmarshal(sanitized, &object))
	assert.Nil(t, object["backgroundColor"])
	series := object["series"].([]any)[0].(map[string]any)
	assert.Empty(t, series["areaStyle"]) // gradient color removed
	yAxis := object["yAxis"].([]any)[0].(map[string]any)
	axisLabel := yAxis["axisLabel"].(map[string]any)
	assert.NotContains(t, axisLabel, "formatter") // non-string formatter removed

	require.Len(t, warnings, 3)
	joined := strings.Join(warnings, "\n")
	assert.Contains(t, joined, "series[0].areaStyle.color")
	assert.Contains(t, joined, "backgroundColor")
	assert.Contains(t, joined, "yAxis[0].axisLabel.formatter")
}

func TestSanitizeOptionStringColorsPass(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"backgroundColor": "#123", "series": [{"type": "line",
		"data": [1], "areaStyle": {"color": "blue"}}]}`)

	sanitized, warnings, err := sanitizeOption(raw)
	require.NoError(t, err)
	assert.JSONEq(t, string(raw), string(sanitized))
	// the gradient-free fill color decodes fine but is documented as parsed-only
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "series[0].areaStyle.color is accepted but not rendered")
}

func TestSanitizeOptionInvalidJSONErrors(t *testing.T) {
	t.Parallel()

	raw := []byte(`{not json`)
	sanitized, warnings, err := sanitizeOption(raw)
	require.ErrorContains(t, err, "parse chart option")
	assert.Nil(t, sanitized)
	assert.Empty(t, warnings)
}

func TestLintIgnoredTopLevel(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"type": "line", "theme": "cyberpunk",
		"series": [{"type": "line", "data": [1]}]}`)

	_, warnings, err := sanitizeOption(raw)
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
	raw := []byte(`{"tooltip": {}, "fontFamily": "roboto",
		"series": [{"type": "line", "data": [1], "gap": 5}],
		"xAxis": {"boundaryGap": false, "data": ["a"]}}`)

	sanitized, warnings, err := sanitizeOption(raw)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.JSONEq(t, string(raw), string(sanitized))
}

func TestLintIgnoredFieldsWarns(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"title": {"text": "t", "backgroundColor": "#fff"},
		"series": [{"type": "line", "data": [1], "max": 9, "areaStyle": {"color": "blue"},
			"markPoint": {"data": [{"xAxis": 3}]}},
			{"type": "bar", "data": [2], "barCategoryGap": "20%"}],
		"xAxis": [{"axisLabel": {"formatter": "{value}"}, "axisLine": {"show": true,
			"lineStyle": {"width": 3}}}]
	}`)

	_, warnings, err := sanitizeOption(raw)
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
		lintDimensions([]byte(`{"series": [], "width": 640}`)))
	assert.Equal(t, []string{"height set without width; defaulted width to 600"},
		lintDimensions([]byte(`{"height": 480}`)))
	assert.Empty(t, lintDimensions([]byte(`{"width": 640, "height": 480}`)))
	assert.Empty(t, lintDimensions([]byte(`{"series": []}`)))
	assert.Empty(t, lintDimensions([]byte(`{invalid`)))
}

func TestEffectiveDimensions(t *testing.T) {
	t.Parallel()

	width, height := effectiveDimensions([]byte(`{"width": 1600}`))
	assert.Equal(t, 1600, width)
	assert.Equal(t, defaultChartHeight, height) // unset dimension keeps its default

	width, height = effectiveDimensions([]byte(`{}`))
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

	raw := []byte(`{
		"series": [
			{"type": "line", "data": [1], "smooth": true},
			{"type": "bar", "data": [2], "symbolSize": 9, "areaStyle": {},
				"lineStyle": {"width": 3}},
			{"type": "line", "data": [3], "smooth": null}
		]
	}`)

	_, warnings, err := sanitizeOption(raw)
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

	raw := []byte(`{
		"xAxis": [{"type": "time", "data": ["a"]}],
		"yAxis": {"type": "log"},
		"series": [
			{"type": "funnel", "label": {"formatter": "{b}: {d}%"}, "data": [1]},
			{"type": "pie", "label": {"formatter": "{d}%"}}
		]
	}`)

	sanitized, warnings, err := sanitizeOption(raw)
	require.NoError(t, err)

	joined := joinWarnings(warnings)
	assert.Contains(t, joined, `xAxis[0].type "time" counts as category; there is no continuous axis`)
	assert.Contains(t, joined, `yAxis[0].type "log" counts as category; there is no continuous axis`)
	assert.Contains(t, joined,
		"series[0].label.formatter {d} is not computed on funnel, removed; {a}, {b}, and {c} still render")
	// pie computes {d}, so it stays untouched
	assert.NotContains(t, joined, "series[1]")

	var object struct {
		Series []struct {
			Label struct {
				Formatter string `json:"formatter"`
			} `json:"label"`
		} `json:"series"`
	}
	require.NoError(t, json.Unmarshal(sanitized, &object))
	assert.Equal(t, "{b}:", object.Series[0].Label.Formatter)
	assert.Equal(t, "{d}%", object.Series[1].Label.Formatter)
}

func TestLintScopeStructures(t *testing.T) {
	t.Parallel()

	t.Run("warns", func(t *testing.T) {
		raw := []byte(`{
			"xAxis": [{"data": ["a"]}, {"data": ["b"], "splitNumber": 4}],
			"series": [{"type": "line", "data": [1]}]
		}`)

		_, warnings, err := sanitizeOption(raw)
		require.NoError(t, err)
		assert.Contains(t, joinWarnings(warnings),
			"xAxis has 2 entries; only the first renders and the rest are ignored")
	})

	t.Run("out of range axis index errors", func(t *testing.T) {
		// silent fallback would draw every series on one scale
		raw := []byte(`{
			"yAxis": [{"name": "n"}],
			"series": [{"type": "line", "data": [1], "yAxisIndex": 3}]
		}`)

		_, _, err := sanitizeOption(raw)
		require.ErrorContains(t, err,
			"series[0].yAxisIndex 3 references a missing yAxis entry (1 declared); add the axis entry or drop the index")
	})

	t.Run("clean", func(t *testing.T) {
		raw := []byte(`{
			"xAxis": [{"data": ["a"]}],
			"yAxis": [{}, {}],
			"series": [{"type": "line", "data": [1], "yAxisIndex": 1}]
		}`)

		sanitized, warnings, err := sanitizeOption(raw)
		require.NoError(t, err)
		assert.Empty(t, warnings)
		assert.JSONEq(t, string(raw), string(sanitized))
	})

	t.Run("default axis index", func(t *testing.T) {
		// yAxisIndex 0 without declared axes keeps the implicit default axis
		raw := []byte(`{"series": [{"type": "line", "data": [1], "yAxisIndex": 0}]}`)

		_, warnings, err := sanitizeOption(raw)
		require.NoError(t, err)
		assert.Empty(t, warnings)
	})
}

func joinWarnings(warnings []string) string {
	return strings.Join(warnings, "\n")
}
