package source

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Column type labels reported by inference.
const (
	TypeNumber = "number"
	TypeBool   = "bool"
	TypeString = "string"
)

// Table is an in-memory tabular dataset.
type Table struct {
	// Columns are the column names in order.
	Columns []string
	// Rows holds one cell per column; short rows read missing cells as nil.
	Rows [][]any
}

// ColumnIndex returns the position of the named column.
func (t *Table) ColumnIndex(name string) (int, bool) {
	for i, col := range t.Columns {
		if col == name {
			return i, true
		}
	}
	return 0, false
}

// ColumnValues returns the cells of column index, padding short rows with nil;
// out-of-range indexes return nil.
func (t *Table) ColumnValues(index int) []any {
	if index < 0 || index >= len(t.Columns) {
		return nil
	}
	values := make([]any, len(t.Rows))
	for i, row := range t.Rows {
		if index < len(row) {
			values[i] = row[index]
		}
	}
	return values
}

// NullCount returns the number of missing cells in column index.
func (t *Table) NullCount(index int) int {
	count := 0
	for _, value := range t.ColumnValues(index) {
		if IsNull(value) {
			count++
		}
	}
	return count
}

// ColumnTypes infers a type label per column from the non-missing cells.
func (t *Table) ColumnTypes() []string {
	types := make([]string, len(t.Columns))
	for i := range t.Columns {
		types[i] = InferColumnType(t.ColumnValues(i))
	}
	return types
}

// IsNull reports whether a cell holds no value: JSON null, an empty string, or
// the ECharts '-' missing-value placeholder.
func IsNull(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == "" || v == "-"
	default:
		return false
	}
}

// InferColumnType labels values number, bool, or string. Every non-missing cell
// must parse as the inferred type; columns with only missing cells fall back to
// string.
func InferColumnType(values []any) string {
	sawValue := false
	allNumber, allBool := true, true
	for _, value := range values {
		if IsNull(value) {
			continue
		}
		sawValue = true
		if _, ok := AsFloat64(value); !ok {
			allNumber = false
		}
		if _, ok := AsBool(value); !ok {
			allBool = false
		}
	}
	switch {
	case sawValue && allBool:
		return TypeBool
	case sawValue && allNumber:
		return TypeNumber
	default:
		return TypeString
	}
}

// AsFloat64 converts a cell to a number, parsing numeric strings.
func AsFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// AsBool converts a cell to a boolean, accepting true/false strings.
func AsBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true":
			return true, true
		case "false":
			return false, true
		}
	}
	return false, false
}

// AsString renders a cell as text.
func AsString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	default:
		return fmt.Sprint(value)
	}
}
