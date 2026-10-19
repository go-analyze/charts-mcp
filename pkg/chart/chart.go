package chart

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/go-analyze/bulk"
	"github.com/go-analyze/charts"

	"github.com/go-analyze/charts-mcp/pkg/source"
)

// Binding keys resolved into native ECharts fields during expansion; every
// other option key passes through to the renderer untouched.
const (
	keySource   = "source"
	keyField    = "field"
	keyColumns  = "columns"
	keyColumn   = "column"
	keyType     = "type"
	keyData     = "data"
	keyName     = "name"
	keyValue    = "value"
	keySeries   = "series"
	keyXAxis    = "xAxis"
	keyYAxis    = "yAxis"
	keyChildren = "children"
)

// namedItemTypes are the series types whose data items carry display names.
var namedItemTypes = []string{
	charts.ChartTypePie, charts.ChartTypeDoughnut, charts.ChartTypeFunnel,
}

// themeNames mirrors the built-in palettes registered by go-analyze/charts.
var themeNames = []string{
	charts.ThemeLight, charts.ThemeDark, charts.ThemeVividLight, charts.ThemeVividDark,
	charts.ThemeGrafana, charts.ThemeAnt, charts.ThemeNatureLight, charts.ThemeNatureDark,
	charts.ThemeRetro, charts.ThemeOcean, charts.ThemeSlate, charts.ThemeGray,
	charts.ThemeWinter, charts.ThemeSpring, charts.ThemeSummer, charts.ThemeFall,
	"default", // library alias for the current default theme
}

// ThemeNames lists the built-in palette names accepted by the theme option.
func ThemeNames() []string {
	return slices.Clone(themeNames)
}

// KnownTheme reports whether name is a built-in palette or its default alias.
func KnownTheme(name string) bool {
	return slices.Contains(themeNames, name)
}

// seriesTypes lists the chart types the renderer's ECharts decoder supports on
// series entries; anything else decodes but renders nothing.
var seriesTypes = []string{
	charts.ChartTypeLine, charts.ChartTypeBar, charts.ChartTypeHorizontalBar,
	charts.ChartTypeScatter, charts.ChartTypePie, charts.ChartTypeDoughnut,
	charts.ChartTypeRadar, charts.ChartTypeFunnel, charts.ChartTypeCandlestick,
	charts.ChartTypeViolin, charts.ChartTypeHorizontalViolin,
}

// SeriesTypes lists the chart types accepted on series[].type.
func SeriesTypes() []string {
	return slices.Clone(seriesTypes)
}

// SeriesSource is the raw source payload of one series entry plus the path
// locating the series within the option document.
type SeriesSource struct {
	Path  string
	Value any
}

// SeriesSources returns the source payload of each series entry in the
// document, in the same order Expand consumes per-series tables. Entries
// without a source carry a nil Value.
func SeriesSources(optionJSON []byte) []SeriesSource {
	var object map[string]any
	if err := json.Unmarshal(optionJSON, &object); err != nil {
		return nil
	}
	refs, err := seriesRefs(object)
	if err != nil {
		return nil
	}
	sources := make([]SeriesSource, len(refs))
	for i, ref := range refs {
		sources[i] = SeriesSource{Path: ref.path, Value: ref.entry[keySource]}
	}
	return sources
}

// Expand resolves the binding keys (series[].field, series[].columns, and the
// axis column keys) against the source tables and returns the expanded document
// plus warnings. Shared is the request-level table; seriesTables holds per-series
// tables in the order SeriesSources lists them and may hold nils. Series errors
// accumulate and identical ones collapse. Errors name the offending series or
// axis so agents can self-correct.
func Expand(optionJSON []byte, shared *source.Table, seriesTables []*source.Table) ([]byte, []string, error) {
	var object map[string]any
	if err := json.Unmarshal(optionJSON, &object); err != nil {
		return nil, nil, fmt.Errorf("parse chart option: %w", err)
	}
	if object[keyChildren] != nil {
		return nil, nil, errors.New(
			"children is not supported; render each chart with its own request and output")
	}
	exp := expander{shared: shared, tables: seriesTables}
	warnings, err := exp.scope(object)
	if err != nil {
		return nil, nil, err
	}
	if shared != nil && !exp.usedShared {
		warnings = append(warnings, "source ignored; no field or column bindings reference it")
	}
	expanded, err := json.Marshal(object)
	if err != nil {
		return nil, nil, err
	}
	return expanded, warnings, nil
}

// expander threads the shared table, per-series tables, and source-usage
// bookkeeping through the recursive document expansion.
type expander struct {
	shared     *source.Table
	tables     []*source.Table
	next       int // next unconsumed per-series table slot
	usedShared bool
}

// scope expands one option document, resolving its axis and series bindings.
func (e *expander) scope(object map[string]any) ([]string, error) {
	labels, axisUsed, axisWarnings, err := e.axes(object)
	if err != nil {
		return nil, err
	}
	e.usedShared = e.usedShared || axisUsed
	refs, err := seriesRefs(object)
	if err != nil {
		return nil, err
	}
	if err := validateRadarIndicators(object, refs); err != nil {
		return nil, err
	}
	warnings := axisWarnings
	var failures []*seriesError
	for _, ref := range refs {
		table := e.shared
		if e.next < len(e.tables) {
			if own := e.tables[e.next]; own != nil {
				table = own
			}
			e.next++
		}
		seriesWarnings, bound, err := expandSeries(ref, labels, table)
		if err != nil {
			var failure *seriesError
			if !errors.As(err, &failure) {
				failure = &seriesError{path: ref.path, msg: err.Error(), key: err.Error()}
			}
			// every series failure shows the node under validation, so a mangled
			// payload is diagnosable in one round trip
			failure.msg += " " + seriesEcho(ref.entry)
			failures = append(failures, failure)
			continue
		}
		warnings = append(warnings, seriesWarnings...)
		// a binding with no per-series table read the shared one
		if bound && table == e.shared {
			e.usedShared = true
		}
	}
	if len(failures) > 0 {
		// the source note separates a payload stripped of its table in transit
		// from an authoring mistake at the call site; identical failures collapse
		return nil, errors.New(joinFailures(failures) + " " + e.sourceNote())
	}
	return warnings, nil
}

// sourceNote summarizes whether a top-level source arrived and what columns it
// carries, for series error context.
func (e *expander) sourceNote() string {
	if e.shared == nil {
		return "(no top-level source present)"
	}
	return "(top-level source present, columns: " + strings.Join(e.shared.Columns, ", ") + ")"
}

// seriesError is one invalid series entry; key names the failure shape so
// entries failing identically collapse into one message.
type seriesError struct {
	path string
	msg  string
	key  string
}

func (e *seriesError) Error() string { return e.msg }

// failf builds a seriesError; format receives the series path as its first verb.
func (ref seriesRef) failf(key, format string, args ...any) *seriesError {
	return &seriesError{
		path: ref.path,
		msg:  fmt.Sprintf(format, append([]any{ref.path}, args...)...),
		key:  key,
	}
}

// wrapSeriesError keys an expansion failure by its message minus the series
// path, so the same column mistake across series collapses.
func wrapSeriesError(ref seriesRef, err error) *seriesError {
	return &seriesError{
		path: ref.path,
		msg:  err.Error(),
		key:  strings.ReplaceAll(err.Error(), ref.path, ""),
	}
}

// joinFailures renders failures in order, collapsing entries sharing a key
// into the first message plus the repeated paths.
func joinFailures(failures []*seriesError) string {
	type group struct {
		message string
		repeats []string
	}
	var groups []*group
	byKey := make(map[string]*group, len(failures))
	for _, failure := range failures {
		if existing, ok := byKey[failure.key]; ok {
			existing.repeats = append(existing.repeats, failure.path)
			continue
		}
		fresh := &group{message: failure.msg}
		byKey[failure.key] = fresh
		groups = append(groups, fresh)
	}
	parts := make([]string, 0, len(groups))
	for _, g := range groups {
		if len(g.repeats) == 0 {
			parts = append(parts, g.message)
			continue
		}
		parts = append(parts, g.message+"; same failure for "+strings.Join(g.repeats, ", "))
	}
	return strings.Join(parts, "; ")
}

// seriesRefs validates and lists the series entries, erroring when the option
// has none or an entry is not an object.
func seriesRefs(object map[string]any) ([]seriesRef, error) {
	items, _ := object[keySeries].([]any)
	if len(items) == 0 {
		return nil, errors.New("option needs at least one series; set series[].type with data, field, or columns")
	}
	refs := make([]seriesRef, len(items))
	for i, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("series[%d] must be an object", i)
		}
		refs[i] = seriesRef{path: fmt.Sprintf("series[%d]", i), entry: entry}
	}
	return refs, nil
}

// axes resolves the xAxis.column and yAxis[].column label bindings, returning
// the first x-axis label list (pie, doughnut, and funnel name their data items
// from it), whether the shared table supplied any axis labels, and warnings.
func (e *expander) axes(object map[string]any) ([]string, bool, []string, error) {
	var labels []string
	used := false
	var warnings []string
	if xAxis := objectList(object[keyXAxis]); len(xAxis) > 0 {
		resolved, axisWarnings, err := resolveAxisColumn("xAxis[0]", xAxis[0], e.shared)
		if err != nil {
			return nil, false, nil, err
		}
		warnings = append(warnings, axisWarnings...)
		if resolved != nil {
			labels = resolved
			used = true
		} else {
			// inline axis labels feed slice naming the same as a bound label column
			labels = axisDataLabels(xAxis[0][keyData])
		}
	}
	for i, entry := range objectList(object[keyYAxis]) {
		resolved, axisWarnings, err := resolveAxisColumn(fmt.Sprintf("yAxis[%d]", i), entry, e.shared)
		if err != nil {
			return nil, false, nil, err
		}
		warnings = append(warnings, axisWarnings...)
		if resolved != nil {
			used = true
		}
	}
	return labels, used, warnings, nil
}

// resolveAxisColumn replaces an axis entry's column binding with its label
// array, returning the labels or nil when no binding is present. A warning
// reports inline data discarded by the expansion.
func resolveAxisColumn(path string, entry map[string]any, shared *source.Table) ([]string, []string, error) {
	if entry[keyColumn] == nil {
		return nil, nil, nil
	}
	column, ok := entry[keyColumn].(string)
	if !ok {
		return nil, nil, fmt.Errorf("%s.column must be a column name string", path)
	}
	if column == "" {
		// stripped payloads can carry empty bindings; treat as unset
		delete(entry, keyColumn)
		return nil, nil, nil
	}
	var warnings []string
	if _, hasInline := entry[keyData]; hasInline && entry[keyData] != nil {
		warnings = append(warnings, path+
			" sets both column and data; the bound column replaces the inline labels")
	}
	labels, err := labelColumn(shared, column)
	if err != nil {
		return nil, nil, fmt.Errorf("%s.column: %w", path, err)
	}
	entry[keyData] = labels
	delete(entry, keyColumn)
	return labels, warnings, nil
}

// axisDataLabels converts an inline axis data array into label strings, matching
// how a bound label column stringifies its cells.
func axisDataLabels(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = source.AsString(item)
	}
	return labels
}

// seriesRef locates one series entry within the option document.
type seriesRef struct {
	path  string
	entry map[string]any
}

// expandSeries resolves one series entry's data binding against table, leaving
// inline data untouched. A series with no data origin at all binds the table's
// sole numeric column implicitly, warned, so partially stripped payloads still
// render. Reports whether a column binding consumed the table.
func expandSeries(ref seriesRef, labels []string, table *source.Table) ([]string, bool, error) {
	series := ref.entry
	seriesType, _ := series[keyType].(string)
	if seriesType == "" {
		return nil, false, ref.failf("missing-type", "%s needs a type; valid types: %s",
			strings.Join(seriesTypes, ", "))
	} else if !slices.Contains(seriesTypes, seriesType) {
		return nil, false, ref.failf("unsupported-type "+seriesType,
			"%s has unsupported type %q; valid types: %s", seriesType, strings.Join(seriesTypes, ", "))
	}
	if series[keyField] != nil {
		if _, ok := series[keyField].(string); !ok {
			return nil, false, ref.failf("field-type", "%s: field must be a column name string")
		}
	}
	columns := objectMap(series[keyColumns])
	if series[keyColumns] != nil && columns == nil {
		return nil, false, ref.failf("columns-shape",
			"%s: columns must be an object of {open, high, low, close} column names")
	}
	// empty strings ride in on stripped payloads; they carry no binding
	field, hasField := "", false
	if text, ok := series[keyField].(string); ok && text != "" {
		field, hasField = text, true
	}
	hasData := series[keyData] != nil

	if hasData && series[keySource] != nil && !hasField && columns == nil {
		return nil, false, ref.failf("data-and-source",
			"%s sets both data and source; a series source only backs field or columns bindings")
	}

	var warnings []string
	if !hasData && !hasField && columns == nil {
		switch seriesType {
		case charts.ChartTypeCandlestick:
			// a lone numeric column would bind into flat [v,v,v,v] candles
			return nil, false, ref.failf("needs-ohlc",
				"%s needs OHLC data; set columns {open, high, low, close} to source columns, "+
					"or pass inline [open, close, lowest, highest] tuples")
		case charts.ChartTypeViolin, charts.ChartTypeHorizontalViolin:
			// single values cannot form extent pairs
			return nil, false, ref.failf("needs-extents",
				"%s needs extent pairs; pass inline [min, max] tuples per category")
		}
		if column, ok := soleNumericColumn(table); ok {
			// deterministic when the table has one numeric column, so no warning
			field, hasField = column, true
		} else if series[keySource] != nil {
			return nil, false, implicitBindingError(ref, table, "its source")
		} else if table != nil {
			return nil, false, implicitBindingError(ref, table, "the top-level source")
		}
	}

	origins := make([]string, 0, 3)
	if hasData {
		origins = append(origins, "data")
	}
	if hasField {
		origins = append(origins, "field")
	}
	if columns != nil {
		origins = append(origins, "columns")
	}
	switch {
	case len(origins) == 0:
		return nil, false, ref.failf("needs-data",
			"%s needs data; set exactly one of data (inline), field, or columns")
	case len(origins) > 1:
		return nil, false, ref.failf("multi-origin "+strings.Join(origins, "+"),
			"%s sets %s; use exactly one data origin", strings.Join(origins, " and "))
	}

	var err error
	if hasField {
		var fieldWarnings []string
		fieldWarnings, err = expandFieldSeries(ref.path, series, seriesType, field, labels, table)
		warnings = append(warnings, fieldWarnings...)
	} else if columns != nil {
		err = expandCandlestickSeries(ref.path, series, columns, table)
	} else if slices.Contains(namedItemTypes, seriesType) {
		warnings = append(warnings, nameInlineItems(ref.path, series, labels)...)
	}
	if err != nil {
		return nil, false, wrapSeriesError(ref, err)
	}
	delete(series, keySource)
	return warnings, hasField || columns != nil, nil
}

// soleNumericColumn reports the table's only numeric column, treating null
// cells as gaps and all-null columns as non-numeric; false when zero or
// several columns qualify, leaving the value column ambiguous.
func soleNumericColumn(table *source.Table) (string, bool) {
	if table == nil {
		return "", false
	}
	types := table.ColumnTypes()
	sole := ""
	for i, name := range table.Columns {
		if types[i] != source.TypeNumber {
			continue
		}
		if sole != "" {
			return "", false
		}
		sole = name
	}
	return sole, sole != ""
}

// implicitBindingError explains why a series with no data origin could not
// bind its table implicitly: no numeric column, or several leaving the choice
// open.
func implicitBindingError(ref seriesRef, table *source.Table, origin string) *seriesError {
	if table == nil {
		return ref.failf("no-table", "%s has no data origin and no table arrived to bind; set data, field, or columns")
	}
	var numeric []string
	types := table.ColumnTypes()
	for i, name := range table.Columns {
		if types[i] == source.TypeNumber {
			numeric = append(numeric, name)
		}
	}
	if len(numeric) == 0 {
		return ref.failf("no-numeric-columns",
			"%s has no data origin and %s has no numeric columns; set field or use inline data", origin)
	}
	return ref.failf("ambiguous-columns",
		"%s has no data origin and %s has %d numeric columns (%s); set field to one",
		origin, len(numeric), strings.Join(numeric, ", "))
}

// expandFieldSeries converts a numeric source column into the data shape the
// series type consumes: scalar items by default, named items for the circular
// chart types, or a single polygon tuple for radar.
func expandFieldSeries(path string, series map[string]any, seriesType, field string, labels []string, table *source.Table) ([]string, error) {
	values, err := numericColumn(table, field)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var warnings []string
	if allNull(values) {
		warnings = append(warnings, fmt.Sprintf(
			"%s column %q has no numeric values; rendering as gaps", path, field))
	}
	name, _ := series[keyName].(string)
	switch seriesType {
	case charts.ChartTypePie, charts.ChartTypeDoughnut, charts.ChartTypeFunnel:
		if name == "" {
			series[keyName] = field
		}
		items := make([]map[string]any, len(values))
		for i, value := range values {
			items[i] = map[string]any{keyValue: value}
			if i < len(labels) {
				items[i][keyName] = labels[i]
			}
		}
		series[keyData] = items
		if len(labels) == 0 {
			warnings = append(warnings, path+" slices have no names; set xAxis data labels or name the data items")
		}
	case charts.ChartTypeRadar:
		if name == "" {
			name = field
			series[keyName] = name
		}
		series[keyData] = []any{map[string]any{keyName: name, keyValue: values}}
	default:
		series[keyData] = values
		if name == "" {
			series[keyName] = field
		}
	}
	delete(series, keyField)
	return warnings, nil
}

// nameInlineItems names pie/doughnut/funnel inline data items from the label
// channel, mirroring field-bound expansion; bare values become {value, name}
// entries. Warns when no slice ends up carrying a display name.
func nameInlineItems(path string, series map[string]any, labels []string) []string {
	items, ok := series[keyData].([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	named := make([]any, len(items))
	changed := false
	for i, item := range items {
		var label any
		if i < len(labels) && labels[i] != "" {
			label = labels[i]
		}
		entry, isObject := item.(map[string]any)
		switch {
		case isObject:
			if _, has := entry[keyName]; !has && label != nil {
				entry[keyName] = label
				changed = true
			}
			named[i] = entry
		case label != nil:
			named[i] = map[string]any{keyValue: item, keyName: label}
			changed = true
		default:
			named[i] = item
		}
	}
	if changed {
		series[keyData] = named
	}
	for _, item := range named {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if name, _ := entry[keyName].(string); name != "" {
			return nil // at least one slice carries a display name
		}
	}
	return []string{path + " slices have no names; set xAxis data labels or name the data items"}
}

// expandCandlestickSeries assembles the four OHLC columns into ECharts
// [open, close, lowest, highest] tuples, one per row.
func expandCandlestickSeries(path string, series map[string]any, columns map[string]any, table *source.Table) error {
	seriesType, _ := series[keyType].(string)
	if seriesType != charts.ChartTypeCandlestick {
		return fmt.Errorf("%s: columns is only supported on candlestick series", path)
	}
	resolved := make(map[string][]any, 4)
	for _, role := range []string{"open", "high", "low", "close"} {
		column, _ := columns[role].(string)
		if column == "" {
			return fmt.Errorf(
				"%s: columns needs all four roles {open, high, low, close} mapping to column names", path)
		}
		values, err := lookupColumn(table, column)
		if err != nil {
			return fmt.Errorf("%s: columns.%s: %w", path, role, err)
		}
		resolved[role] = values
	}
	data := make([]any, len(resolved["open"]))
	for i := range data {
		tuple := make([]any, 4)
		for j, role := range []string{"open", "close", "low", "high"} { // ECharts tuple order
			cell := resolved[role][i]
			if source.IsNull(cell) {
				return fmt.Errorf(
					"%s: columns.%s has a missing value at row %d; candlesticks need four numbers", path, role, i+1)
			}
			value, ok := source.AsFloat64(cell)
			if !ok {
				return fmt.Errorf("%s: columns.%s has a non-numeric value at row %d",
					path, role, i+1)
			}
			tuple[j] = value
		}
		data[i] = tuple
	}
	series[keyData] = data
	delete(series, keyColumns)
	return nil
}

// seriesEcho renders a series entry compactly for error context, so a
// stripped or misshapen payload is diagnosable without echoing whole datasets:
// scalar values verbatim, arrays and objects summarized to their shape.
func seriesEcho(series map[string]any) string {
	keys := bulk.MapKeysSlice(series)
	slices.Sort(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, strconv.Quote(key)+":"+echoValue(series[key]))
	}
	return "(received: {" + strings.Join(parts, ",") + "})"
}

// echoValue summarizes one entry value: scalars verbatim, collections as their
// size, anything unexpected by type.
func echoValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return strconv.Quote(typed)
	case bool:
		return strconv.FormatBool(typed)
	case int, int32, int64, float32, float64:
		return fmt.Sprint(typed)
	case []any:
		return fmt.Sprintf("[%d items]", len(typed))
	case map[string]any:
		return fmt.Sprintf("{%d keys}", len(typed))
	default:
		return fmt.Sprintf("<%T>", typed)
	}
}

// validateRadarIndicators errors when a scope renders radar without the
// indicator block the renderer requires, pointing at radar.indicator instead of
// surfacing a raw renderer error after expansion.
func validateRadarIndicators(object map[string]any, refs []seriesRef) error {
	var path string
	for _, ref := range refs {
		if seriesType, _ := ref.entry[keyType].(string); seriesType == charts.ChartTypeRadar {
			path = ref.path
			break
		}
	}
	if path == "" {
		return nil
	}
	radar := objectMap(object["radar"])
	indicators := objectList(radar["indicator"])
	if len(indicators) < 3 {
		return fmt.Errorf("%s needs a radar.indicator block with at least three indicators; "+
			"category labels do not build radar spokes", path)
	}
	return nil
}

// allNull reports whether every value is the gap sentinel.
func allNull(values []any) bool {
	if len(values) == 0 {
		return false
	}
	null := charts.GetNullValue()
	for _, value := range values {
		if number, ok := value.(float64); !ok || number != null {
			return false
		}
	}
	return true
}

// numericColumn converts a table column into numeric values; missing, empty,
// and '-' cells map to the library null sentinel and render as gaps.
func numericColumn(table *source.Table, name string) ([]any, error) {
	values, err := lookupColumn(table, name)
	if err != nil {
		return nil, err
	}
	numbers := make([]any, len(values))
	for i, cell := range values {
		if source.IsNull(cell) {
			numbers[i] = charts.GetNullValue()
			continue
		}
		value, ok := source.AsFloat64(cell)
		if !ok {
			return nil, fmt.Errorf("column %q has a non-numeric value at row %d: %q",
				name, i+1, source.AsString(cell))
		}
		numbers[i] = value
	}
	return numbers, nil
}

// labelColumn converts a table column into label strings.
func labelColumn(table *source.Table, name string) ([]string, error) {
	values, err := lookupAxisColumn(table, name)
	if err != nil {
		return nil, err
	}
	labels := make([]string, len(values))
	for i, cell := range values {
		labels[i] = source.AsString(cell)
	}
	return labels, nil
}

// lookupColumn resolves a column name against the table backing a series
// binding; the top-level source or this series' own source may supply it.
func lookupColumn(table *source.Table, name string) ([]any, error) {
	if table == nil {
		return nil, fmt.Errorf("column %q requires a data source; set the top-level source or "+
			"this series' own source, or switch to inline data", name)
	}
	index, ok := table.ColumnIndex(name)
	if !ok {
		return nil, fmt.Errorf("column %q not found%s; available columns: %s",
			name, caseHint(name, table.Columns), strings.Join(table.Columns, ", "))
	}
	return table.ColumnValues(index), nil
}

// lookupAxisColumn resolves a column name against the top-level source backing
// an axis binding; axes never read per-series sources.
func lookupAxisColumn(table *source.Table, name string) ([]any, error) {
	if table == nil {
		return nil, fmt.Errorf("column %q requires a data source; no top-level source is present "+
			"and axis column bindings never read per-series sources. Set the top-level source or "+
			"supply inline labels through axis data", name)
	}
	index, ok := table.ColumnIndex(name)
	if !ok {
		return nil, fmt.Errorf("column %q not found in the top-level source%s; available columns: %s",
			name, caseHint(name, table.Columns), strings.Join(table.Columns, ", "))
	}
	return table.ColumnValues(index), nil
}

// caseHint names a differing-case column match, the most common near miss.
func caseHint(name string, columns []string) string {
	for _, column := range columns {
		if strings.EqualFold(column, name) {
			return fmt.Sprintf(" (a %q column exists; column names are case-sensitive)", column)
		}
	}
	return ""
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

// objectMap returns value as an object, or nil when it is anything else.
func objectMap(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}
