package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTableColumnIndex(t *testing.T) {
	t.Parallel()

	table := &Table{Columns: []string{"month", "revenue"}}

	index, ok := table.ColumnIndex("revenue")
	assert.True(t, ok)
	assert.Equal(t, 1, index)

	_, ok = table.ColumnIndex("missing")
	assert.False(t, ok)
}

func TestTableColumnValues(t *testing.T) {
	t.Parallel()

	table := &Table{
		Columns: []string{"a", "b"},
		Rows:    [][]any{{1, "x"}, {2}, {3, "y"}},
	}

	assert.Equal(t, []any{1, 2, 3}, table.ColumnValues(0))
	assert.Equal(t, []any{"x", nil, "y"}, table.ColumnValues(1))
	assert.Nil(t, table.ColumnValues(-1))
	assert.Nil(t, table.ColumnValues(5))
}

func TestTableNullCount(t *testing.T) {
	t.Parallel()

	table := &Table{
		Columns: []string{"a"},
		Rows:    [][]any{{nil}, {""}, {"x"}, {}},
	}

	assert.Equal(t, 3, table.NullCount(0))
}

func TestTableColumnTypes(t *testing.T) {
	t.Parallel()

	table := &Table{
		Columns: []string{"month", "revenue", "active", "blank"},
		Rows: [][]any{
			{"Jan", 120, true, nil},
			{"Feb", "145", false, ""},
		},
	}

	assert.Equal(t, []string{TypeString, TypeNumber, TypeBool, TypeString}, table.ColumnTypes())
}

func TestInferColumnType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		values []any
		want   string
	}{
		{"numbers", []any{1, "2.5", nil}, TypeNumber},
		{"bools", []any{true, "false"}, TypeBool},
		{"mixed is string", []any{"a", 1}, TypeString},
		{"no values is string", nil, TypeString},
		{"all null is string", []any{nil, ""}, TypeString},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, InferColumnType(tc.values))
		})
	}
}

func TestAsFloat64(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		want  float64
		ok    bool
	}{
		{"float", float64(1.5), 1.5, true},
		{"int", 3, 3, true},
		{"numeric string", "2.5", 2.5, true},
		{"padded string", " 4 ", 4, true},
		{"bool rejected", true, 0, false},
		{"text rejected", "abc", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := AsFloat64(tc.value)
			assert.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.InDelta(t, tc.want, got, 0.0001)
			}
		})
	}
}

func TestAsBool(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		want  bool
		ok    bool
	}{
		{"true", true, true, true},
		{"yes rejected", "yes", false, false},
		{"upper true", "TRUE", true, true},
		{"false string", "false", false, true},
		{"number rejected", 1, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := AsBool(tc.value)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.ok, ok)
		})
	}
}

func TestAsString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"nil", nil, ""},
		{"string", "a", "a"},
		{"bool", true, "true"},
		{"float", 1.5, "1.5"},
		{"int", -3, "-3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, AsString(tc.value))
		})
	}
}
