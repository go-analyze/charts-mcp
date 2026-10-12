package chart

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateModes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		spec    Spec
		wantErr string
	}{
		{"semantic", Spec{Type: TypeLine, X: "month"}, ""},
		{"raw option", Spec{Option: json.RawMessage(`{"xAxis": {}}`)}, ""},
		{"option file", Spec{OptionFile: "option.json"}, ""},
		{
			"both raw forms",
			Spec{Option: json.RawMessage("{}"), OptionFile: "o.json"},
			"use exactly one",
		},
		{
			"raw plus semantic",
			Spec{Type: TypeBar, Option: json.RawMessage("{}")},
			"set one or the other",
		},
		{"empty spec", Spec{}, "chart is empty"},
		{"null option absent", Spec{Option: json.RawMessage("null")}, "chart is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.spec.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestValidateTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		spec    Spec
		wantErr string
	}{
		{"type required", Spec{X: "month"}, "chart type is required"},
		{
			"unknown type",
			Spec{Type: "scatter"},
			"valid types: line, bar, pie, radar, funnel, candlestick",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorContains(t, tc.spec.Validate(), tc.wantErr)
		})
	}
}

func TestValidateSeriesItems(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		spec    Spec
		wantErr string
	}{
		{
			"field and values",
			Spec{Type: TypeLine, Series: []Series{{Field: "a", Values: []any{1}}}},
			"use one or the other",
		},
		{
			"candlestick field mapping",
			Spec{Type: TypeCandlestick, Series: []Series{{Field: "open"}}},
			"tuples in values",
		},
		{
			"candlestick values",
			Spec{Type: TypeCandlestick, Series: []Series{{Values: []any{[]any{1, 2, 3, 4}}}}},
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.spec.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestRawReportsEscapeHatch(t *testing.T) {
	t.Parallel()

	assert.False(t, Spec{Type: TypePie}.Raw())
	assert.True(t, Spec{OptionFile: "o.json"}.Raw())
	assert.True(t, Spec{Option: json.RawMessage("{}")}.Raw())
}

func TestValidTypesListsAll(t *testing.T) {
	t.Parallel()
	assert.Equal(t, allTypes, ValidTypes())

	ValidTypes()[0] = "mutated"
	assert.Equal(t, TypeLine, ValidTypes()[0])
}
