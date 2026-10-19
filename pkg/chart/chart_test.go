package chart

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-analyze/charts-mcp/pkg/source"
)

// table builds a table from column names and object rows.
func table(t *testing.T, columns []string, rows ...[]any) *source.Table {
	t.Helper()
	built, err := source.FromColumns(columns, rows)
	require.NoError(t, err)
	return built
}

func TestExpandFieldToData(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"month", "revenue"},
		[]any{"Jan", 120.0}, []any{"Feb", 90.0}, []any{"Mar", nil})

	option := []byte(`{"theme": "dark",
		"xAxis": {"column": "month"},
		"series": [{"type": "line", "field": "revenue"}]}`)

	expanded, warnings, err := Expand(option, shared, nil)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.JSONEq(t, `{"theme": "dark",
		"xAxis": {"data": ["Jan", "Feb", "Mar"]},
		"series": [{"type": "line", "name": "revenue", "data": [120, 90, 1.7976931348623157e+308]}]}`,
		string(expanded))
}

func TestExpandFieldNamesPieSlicesFromAxis(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"channel", "visits"},
		[]any{"Direct", 335.0}, []any{"Email", 310.0})

	option := []byte(`{"xAxis": {"column": "channel"},
		"series": [{"type": "pie", "field": "visits"}]}`)

	expanded, warnings, err := Expand(option, shared, nil)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.JSONEq(t, `{"xAxis": {"data": ["Direct", "Email"]},
		"series": [{"type": "pie", "name": "visits", "data": [
			{"value": 335, "name": "Direct"}, {"value": 310, "name": "Email"}]}]}`,
		string(expanded))
}

func TestExpandPieWithoutLabelsWarns(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"visits"}, []any{335.0}, []any{310.0})
	option := []byte(`{"series": [{"type": "pie", "field": "visits"}]}`)

	_, warnings, err := Expand(option, shared, nil)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "series[0] slices have no names")
}

// TestExpandInline covers series[].data without source bindings: passthrough
// shapes, pie/doughnut/funnel slice naming from the label channel, and the
// no-names warning.
func TestExpandInline(t *testing.T) {
	t.Parallel()

	t.Run("keeps_data", func(t *testing.T) {
		option := []byte(`{"title": {"text": "t"},
			"series": [{"type": "bar", "name": "A", "data": [1, 2, null]}]}`)

		expanded, warnings, err := Expand(option, nil, nil)
		require.NoError(t, err)
		assert.Empty(t, warnings)
		assert.JSONEq(t, string(option), string(expanded))
	})

	t.Run("named_from_axis", func(t *testing.T) {
		option := []byte(`{"xAxis": {"data": ["Chrome", "Safari"]},
			"series": [{"type": "pie", "name": "Share", "data": [1048, 735]}]}`)

		expanded, warnings, err := Expand(option, nil, nil)
		require.NoError(t, err)
		assert.Empty(t, warnings)
		assert.JSONEq(t, `{"xAxis": {"data": ["Chrome", "Safari"]},
			"series": [{"type": "pie", "name": "Share", "data": [
				{"value": 1048, "name": "Chrome"}, {"value": 735, "name": "Safari"}]}]}`,
			string(expanded))
	})

	t.Run("keeps_item_names", func(t *testing.T) {
		option := []byte(`{"series": [{"type": "pie", "data": [
			{"value": 40, "name": "direct"}, {"value": 25, "name": "search"}]}]}`)

		expanded, warnings, err := Expand(option, nil, nil)
		require.NoError(t, err)
		assert.Empty(t, warnings) // pre-named items stay untouched, no label warnings
		assert.JSONEq(t, string(option), string(expanded))
	})

	t.Run("without_labels_warns", func(t *testing.T) {
		_, warnings, err := Expand([]byte(`{"series": [{"type": "funnel", "data": [60, 100]}]}`), nil, nil)
		require.NoError(t, err)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "series[0] slices have no names")
	})
}

func TestExpandRadarFieldBecomesPolygonTuple(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"speed", "power", "range"},
		[]any{5.0, 7.0, 6.0})
	option := []byte(`{"radar": {"indicator": [{"name": "speed"}, {"name": "power"}, {"name": "range"}]},
		"series": [{"type": "radar", "name": "A", "field": "speed"}]}`)

	expanded, warnings, err := Expand(option, shared, nil)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.JSONEq(t, `{"radar": {"indicator": [{"name": "speed"}, {"name": "power"}, {"name": "range"}]},
		"series": [{"type": "radar", "name": "A", "data": [{"name": "A", "value": [5]}]}]}`,
		string(expanded))
}

func TestExpandRadarColumnPolygons(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"team", "speed", "power"},
		[]any{"A", 5.0, 7.0}, []any{"B", 4.0, 8.0})
	option := []byte(`{"radar": {"indicator": [{"name": "x"}, {"name": "y"}, {"name": "z"}]},
		"series": [
			{"type": "radar", "field": "speed"},
			{"type": "radar", "field": "power"}]}`)

	expanded, warnings, err := Expand(option, shared, nil)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.JSONEq(t, `{"radar": {"indicator": [{"name": "x"}, {"name": "y"}, {"name": "z"}]},
		"series": [
			{"type": "radar", "name": "speed", "data": [{"name": "speed", "value": [5, 4]}]},
			{"type": "radar", "name": "power", "data": [{"name": "power", "value": [7, 8]}]}]}`,
		string(expanded))
}

func TestExpandCandlestickColumns(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"o", "h", "l", "c"},
		[]any{10.0, 15.0, 9.0, 14.0}, []any{14.0, 16.0, 11.0, 12.0})
	option := []byte(`{"series": [{"type": "candlestick", "columns": {
		"open": "o", "high": "h", "low": "l", "close": "c"}}]}`)

	expanded, warnings, err := Expand(option, shared, nil)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	// tuples assemble in ECharts order [open, close, lowest, highest]
	assert.JSONEq(t, `{"series": [{"type": "candlestick",
		"data": [[10, 14, 9, 15], [14, 12, 11, 16]]}]}`,
		string(expanded))
}

func TestExpandPerSeriesSourceOverridesShared(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"val"}, []any{1.0}, []any{2.0})
	own := table(t, []string{"num"}, []any{7.0}, []any{8.0})
	option := []byte(`{"series": [
		{"type": "bar", "field": "val"},
		{"type": "bar", "field": "num", "source": {"file": "own.csv"}}]}`)

	expanded, warnings, err := Expand(option, shared, []*source.Table{nil, own})
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.JSONEq(t, `{"series": [
		{"type": "bar", "name": "val", "data": [1, 2]},
		{"type": "bar", "name": "num", "data": [7, 8]}]}`,
		string(expanded))
}

func TestExpandYAxisColumnLabels(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"stage", "days"},
		[]any{"build", 1.0}, []any{"test", 4.0})
	option := []byte(`{"yAxis": [{"type": "category", "column": "stage"}],
		"series": [{"type": "bar", "field": "days"}]}`)

	expanded, _, err := Expand(option, shared, nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"yAxis": [{"type": "category", "data": ["build", "test"]}],
		"series": [{"type": "bar", "name": "days", "data": [1, 4]}]}`,
		string(expanded))
}

func TestExpandUnusedSourceWarns(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"val"}, []any{1.0})
	option := []byte(`{"series": [{"type": "line", "data": [1, 2]}]}`)

	_, warnings, err := Expand(option, shared, nil)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "source ignored")
}

func TestExpandAllNullColumnWarns(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"v"}, []any{nil}, []any{"-"})
	option := []byte(`{"series": [{"type": "line", "field": "v"}]}`)

	_, warnings, err := Expand(option, shared, nil)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], `column "v" has no numeric values`)
}

func TestExpandErrors(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"month", "revenue"},
		[]any{"Jan", 120.0}, []any{"Feb", 90.0})

	cases := []struct {
		name    string
		option  string
		wantErr string
	}{
		{"missing series", `{"title": {"text": "t"}}`, "option needs at least one series"},
		{"empty series", `{"series": []}`, "option needs at least one series"},
		{"series not object", `{"series": ["junk"]}`, "series[0] must be an object"},
		{
			"data and field", `{"series": [{"type": "line", "data": [1], "field": "revenue"}]}`,
			"sets data and field",
		},
		{
			"source with inline data", `{"series": [{"type": "line", "data": [1], "source": {"file": "x"}}]}`,
			"sets both data and source; a series source only backs field or columns bindings",
		},
		{
			"missing column", `{"series": [{"type": "line", "field": "profit"}]}`,
			"column \"profit\" not found; available columns: month, revenue",
		},
		{
			"column case mismatch", `{"series": [{"type": "line", "field": "Revenue"}]}`,
			`column "Revenue" not found (a "revenue" column exists; column names are case-sensitive)`,
		},
		{
			"non-numeric column", `{"series": [{"type": "line", "field": "month"}]}`,
			"has a non-numeric value at row 1",
		},
		{
			"axis column missing", `{"xAxis": {"column": "nope"}, "series": [{"type": "line", "data": [1]}]}`,
			"xAxis[0].column: column \"nope\" not found",
		},
		{
			"columns on non-candlestick", `{"series": [{"type": "line", "columns": {"open": "o", "high": "h", "low": "l", "close": "c"}}]}`,
			"only supported on candlestick series",
		},
		{
			"columns missing role", `{"series": [{"type": "candlestick", "columns": {"open": "month"}}]}`,
			"needs all four roles",
		},
		{
			"radar without indicators", `{"series": [{"type": "radar", "data": [[1, 2, 3]]}]}`,
			"series[0] needs a radar.indicator block with at least three indicators",
		},
		{
			"radar too few indicators",
			`{"radar": {"indicator": [{"name": "x"}]}, "series": [{"type": "radar", "data": [[1]]}]}`,
			"at least three indicators",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Expand([]byte(tc.option), shared, nil)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}

	// a field binding with no table anywhere names the missing source
	_, _, err := Expand([]byte(`{"series": [{"type": "line", "field": "revenue"}]}`), nil, nil)
	require.ErrorContains(t, err,
		`requires a data source; set the top-level source or this series' own source, or switch to inline data`)

	// an axis label binding states that no top-level source arrived and offers
	// the inline alternative so the suggested remedy actually works
	_, _, err = Expand([]byte(
		`{"xAxis": {"column": "revenue"}, "series": [{"type": "line", "data": [1]}]}`), nil, nil)
	require.ErrorContains(t, err,
		"no top-level source is present and axis column bindings never read per-series sources")
	assert.Contains(t, err.Error(), "supply inline labels through axis data")

	// a declared series source whose table never loaded says so plainly
	_, _, err = Expand([]byte(
		`{"series": [{"type": "line", "name": "A", "source": {"file": "x.csv"}}]}`), nil, nil)
	require.ErrorContains(t, err, "no data origin and no table arrived to bind")
}

func TestExpandSeriesErrorsCarrySourceNote(t *testing.T) {
	t.Parallel()

	// ambiguity keeps the needs-data class of failure reachable with a shared table
	shared := table(t, []string{"month", "revenue", "profit"}, []any{"Jan", 1.0, 2.0})
	option := []byte(`{"series": [{"type": "line", "name": "A"}]}`)

	_, _, err := Expand(option, shared, nil)
	require.ErrorContains(t, err, "(top-level source present, columns: month, revenue, profit)")

	_, _, err = Expand(option, nil, nil)
	require.ErrorContains(t, err, "(no top-level source present)")
}

func TestExpandImplicitBindingOwnSource(t *testing.T) {
	t.Parallel()

	own := table(t, []string{"month", "hits"}, []any{"Jan", 7.0}, []any{"Feb", 8.0})
	option := []byte(`{"series": [{"type": "line", "name": "Hits", "source": {"file": "own.csv"}}]}`)

	expanded, warnings, err := Expand(option, nil, []*source.Table{own})
	require.NoError(t, err)
	assert.Empty(t, warnings) // the sole numeric column is deterministic, so binding is silent
	assert.JSONEq(t, `{"series": [{"type": "line", "name": "Hits", "data": [7, 8]}]}`,
		string(expanded))
}

func TestExpandImplicitBindingSharedSource(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"month", "revenue"},
		[]any{"Jan", 120.0}, []any{"Feb", 90.0})
	option := []byte(`{"series": [{"type": "bar", "name": "Revenue"}]}`)

	expanded, warnings, err := Expand(option, shared, nil)
	require.NoError(t, err)
	assert.Empty(t, warnings) // the binding consumes the source, so no ignored note either
	assert.JSONEq(t, `{"series": [{"type": "bar", "name": "Revenue", "data": [120, 90]}]}`,
		string(expanded))
}

func TestExpandImplicitBindingExcludesStructuredTypes(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"day", "close"}, []any{"Mon", 1.5})
	_, _, err := Expand([]byte(`{"series": [{"type": "candlestick", "name": "px"}]}`), shared, nil)
	require.ErrorContains(t, err,
		"series[0] needs OHLC data; set columns {open, high, low, close} to source columns")

	_, _, err = Expand([]byte(`{"series": [{"type": "violin", "name": "v"}]}`), shared, nil)
	require.ErrorContains(t, err, "series[0] needs extent pairs; pass inline [min, max] tuples")
}

func TestExpandImplicitBindingAmbiguous(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"month", "revenue", "profit"}, []any{"Jan", 120.0, 40.0})
	_, _, err := Expand([]byte(`{"series": [{"type": "line", "name": "A"}]}`), shared, nil)
	require.ErrorContains(t, err,
		"series[0] has no data origin and the top-level source has 2 numeric columns (revenue, profit); set field to one")

	own := table(t, []string{"label"}, []any{"Jan"})
	_, _, err = Expand([]byte(`{"series": [{"type": "line", "source": {"file": "own.csv"}}]}`),
		nil, []*source.Table{own})
	require.ErrorContains(t, err, "has no numeric columns; set field or use inline data")
}

func TestExpandAggregatesIdenticalFailures(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"month", "revenue", "profit"}, []any{"Jan", 1.0, 2.0})
	option := []byte(`{"series": [
		{"type": "line", "name": "A"},
		{"type": "line", "name": "B"},
		{"type": "bar", "name": "C"},
		{"type": "pie", "data": [1, 2]}]}`)

	_, _, err := Expand(option, shared, nil)
	require.Error(t, err)
	message := err.Error()
	assert.Contains(t, message, "series[0] has no data origin and the top-level source has 2 numeric columns")
	assert.Contains(t, message, "; same failure for series[1], series[2]")
	assert.NotContains(t, message, "series[1] has no data origin")
}

func TestExpandEmptyBindingsTreatedAsUnset(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"month", "revenue"},
		[]any{"Jan", 120.0}, []any{"Feb", 90.0})
	option := []byte(`{"xAxis": {"column": "", "data": ["A", "B"]},
		"series": [{"type": "line", "name": "A", "field": ""}]}`)

	expanded, warnings, err := Expand(option, shared, nil)
	require.NoError(t, err)
	assert.Empty(t, warnings) // empty bindings bind silently too
	assert.JSONEq(t, `{"xAxis": {"data": ["A", "B"]},
		"series": [{"type": "line", "name": "A", "data": [120, 90]}]}`,
		string(expanded))
}

func TestSeriesEchoSummarizesCollections(t *testing.T) {
	t.Parallel()

	// scalar keys stay verbatim so a sender can diff intent
	full := seriesEcho(map[string]any{"type": "line", "name": "Visits", "yAxisIndex": 0})
	assert.Equal(t, `(received: {"name":"Visits","type":"line","yAxisIndex":0})`, full)

	// collections summarize to their size instead of echoing whole datasets
	var long []any
	for i := 0; i < 700; i++ {
		long = append(long, 1.0)
	}
	echo := seriesEcho(map[string]any{"type": "line", "data": long,
		"label": map[string]any{"show": true, "color": "red"}})
	assert.Contains(t, echo, `"data":[700 items]`)
	assert.Contains(t, echo, `"label":{2 keys}`)
	assert.Less(t, len(echo), 120)
}

func TestExpandAxisColumnReplacesInlineData(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"month", "revenue"},
		[]any{"Jan", 120.0}, []any{"Feb", 90.0})
	option := []byte(`{"xAxis": {"column": "month", "data": ["A", "B"]},
		"series": [{"type": "line", "field": "revenue"}]}`)

	expanded, warnings, err := Expand(option, shared, nil)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "xAxis[0] sets both column and data; the bound column replaces the inline labels")
	assert.JSONEq(t, `{"xAxis": {"data": ["Jan", "Feb"]},
		"series": [{"type": "line", "name": "revenue", "data": [120, 90]}]}`,
		string(expanded))
}

func TestExpandCandlestickMissingCellErrors(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"o", "h", "l", "c"},
		[]any{10.0, 15.0, nil, 14.0})
	option := []byte(`{"series": [{"type": "candlestick", "columns": {
		"open": "o", "high": "h", "low": "l", "close": "c"}}]}`)

	_, _, err := Expand(option, shared, nil)
	assert.ErrorContains(t, err, "columns.low has a missing value at row 1")
}

func TestExpandRejectsChildren(t *testing.T) {
	t.Parallel()

	shared := table(t, []string{"v"}, []any{1.0})
	option := []byte(`{"series": [{"type": "line", "data": [1]}],
		"children": [{"series": [{"type": "bar", "field": "v"}]}]}`)

	_, _, err := Expand(option, shared, nil)
	require.ErrorContains(t, err, "children is not supported")
}

func TestExpandAccumulatesSeriesErrors(t *testing.T) {
	t.Parallel()

	option := []byte(`{"series": [
		{"type": "line", "name": "no origin"},
		{"type": "bar", "data": [1], "field": "v"}]}`)

	_, _, err := Expand(option, nil, nil)
	require.ErrorContains(t, err, "series[0] needs data")
	assert.ErrorContains(t, err, "series[1] sets data and field")
}

func TestExpandSeriesTypeValidation(t *testing.T) {
	t.Parallel()

	_, _, err := Expand([]byte(`{"series": [{"type": "heatMap", "data": [[1, 2]]}]}`), nil, nil)
	require.ErrorContains(t, err, `series[0] has unsupported type "heatMap"; valid types:`)

	_, _, err = Expand([]byte(`{"series": [{"data": [1]}]}`), nil, nil)
	assert.ErrorContains(t, err, "series[0] needs a type; valid types:")
}

func TestSeriesTypesList(t *testing.T) {
	t.Parallel()

	types := SeriesTypes()
	assert.Contains(t, types, "horizontalViolin")
	types[0] = "mutated"
	assert.NotEqual(t, types, SeriesTypes()) // callers get a clone
}

func TestSeriesSourcesIndexesSeries(t *testing.T) {
	t.Parallel()

	sources := SeriesSources([]byte(
		`{"series": [{"type": "line", "data": [1]}, {"type": "line", "field": "v", "source": {"file": "a.csv"}}]}`))
	require.Len(t, sources, 2)
	assert.Equal(t, "series[0]", sources[0].Path)
	assert.Nil(t, sources[0].Value)
	assert.Equal(t, "series[1]", sources[1].Path)
	assert.Equal(t, map[string]any{"file": "a.csv"}, sources[1].Value)

	assert.Empty(t, SeriesSources([]byte(`{"xAxis": {}}`)))
}

func TestKnownTheme(t *testing.T) {
	t.Parallel()

	assert.True(t, KnownTheme("dark"))
	assert.True(t, KnownTheme("default"))
	assert.False(t, KnownTheme("cyberpunk"))

	names := ThemeNames()
	assert.Contains(t, names, "light")
	names[0] = "mutated"
	assert.NotEqual(t, names, ThemeNames()) // callers get a clone
}
