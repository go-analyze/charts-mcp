// Package chart defines the semantic chart specification shared by the tools:
// request structs mirroring the render_chart chart schema plus validation.
package chart

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Semantic v0 chart types. Scatter and heat map stay excluded; scatter remains
// reachable through the option escape hatch.
const (
	TypeLine        = "line"
	TypeBar         = "bar"
	TypePie         = "pie"
	TypeRadar       = "radar"
	TypeFunnel      = "funnel"
	TypeCandlestick = "candlestick"
)

// allTypes backs ValidTypes and type validation.
var allTypes = []string{TypeLine, TypeBar, TypePie, TypeRadar, TypeFunnel, TypeCandlestick}

// ValidTypes lists the semantic chart types accepted in v0.
func ValidTypes() []string {
	return slices.Clone(allTypes)
}

// Series is one plotted series: a source column mapping (Field) or raw
// pre-shaped values, never both. Candlestick requires Values tuples in ECharts
// order [open, close, lowest, highest].
type Series struct {
	Field  string `json:"field,omitempty"`
	Name   string `json:"name,omitempty"`
	Values []any  `json:"values,omitempty"`
}

// Spec is the semantic chart specification, mirroring the render_chart chart
// schema. Option and OptionFile are escape hatches; exactly one of them or the
// semantic fields must be present.
type Spec struct {
	Type       string   `json:"type,omitempty"`
	Title      string   `json:"title,omitempty"`
	Subtitle   string   `json:"subtitle,omitempty"`
	X          string   `json:"x,omitempty"`
	Series     []Series `json:"series,omitempty"`
	Theme      string   `json:"theme,omitempty"`
	Background string   `json:"background,omitempty"`

	Option     json.RawMessage `json:"option,omitempty"`
	OptionFile string          `json:"option_file,omitempty"`
}

// Raw reports whether the spec uses an escape hatch form instead of semantic
// fields.
func (s Spec) Raw() bool {
	return s.hasInlineOption() || s.OptionFile != ""
}

// Validate checks chart spec invariants: mode exclusivity between the escape
// hatch forms and the semantic fields, plus a known type and sane series items
// for semantic specs.
func (s Spec) Validate() error {
	hasOption := s.hasInlineOption()
	if hasOption && s.OptionFile != "" {
		return errors.New("chart sets both option and option_file; use exactly one")
	}
	raw, semantic := hasOption || s.OptionFile != "", s.hasSemanticFields()
	switch {
	case raw && semantic:
		return errors.New("chart mixes the option/option_file escape hatch with semantic fields; set one or the other")
	case raw:
		return nil
	case !semantic:
		return errors.New("chart is empty; provide semantic fields or one of option / option_file")
	}
	if s.Type == "" {
		return fmt.Errorf("chart type is required; valid types: %s", strings.Join(allTypes, ", "))
	} else if !slices.Contains(allTypes, s.Type) {
		return fmt.Errorf("unknown chart type %q; valid types: %s", s.Type, strings.Join(allTypes, ", "))
	}
	for i, series := range s.Series {
		if series.Field != "" && series.Values != nil {
			return fmt.Errorf("series %d sets both field and values; use one or the other", i)
		} else if s.Type == TypeCandlestick && series.Field != "" {
			return fmt.Errorf(
				"candlestick series %d needs [open, close, lowest, highest] tuples in values; field mapping is not supported",
				i)
		}
	}
	return nil
}

// hasInlineOption reports whether the spec carries a non-null inline option.
func (s Spec) hasInlineOption() bool {
	return len(s.Option) > 0 && string(s.Option) != "null"
}

// hasSemanticFields reports whether any semantic field is present.
func (s Spec) hasSemanticFields() bool {
	return s.Type != "" || s.Title != "" || s.Subtitle != "" || s.X != "" ||
		len(s.Series) > 0 || s.Theme != "" || s.Background != ""
}
