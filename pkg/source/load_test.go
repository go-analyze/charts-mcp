package source

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadInlineForms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		source map[string]any
	}{
		{"csv text", map[string]any{"csv": "month,revenue\nJan,120\nFeb,145\n"}},
		{
			"columns rows",
			map[string]any{
				"columns": []any{"month", "revenue"},
				"rows":    []any{[]any{"Jan", 120}, []any{"Feb", 145}},
			},
		},
		{
			"object rows",
			map[string]any{
				"rows": []any{
					map[string]any{"month": "Jan", "revenue": 120},
					map[string]any{"month": "Feb", "revenue": 145},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table, err := Load("", false, tc.source)
			require.NoError(t, err)
			assert.Equal(t, []string{"month", "revenue"}, table.Columns)
			// CSV cells stay strings; typed access goes through the As* helpers.
			if tc.name != "csv text" {
				assert.Equal(t, []any{"Jan", 120}, table.Rows[0])
				assert.Equal(t, []any{"Feb", 145}, table.Rows[1])
			} else {
				assert.Equal(t, []any{"Jan", "120"}, table.Rows[0])
				assert.Equal(t, []any{"Feb", "145"}, table.Rows[1])
			}
		})
	}
}

func TestLoadFileForm(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "data.csv"), []byte("a,b\n1,x\n"), 0o600))

	table, err := Load(root, false, map[string]any{"file": "data.csv"})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, table.Columns)

	_, err = Load("", true, map[string]any{"file": filepath.Join(root, "data.csv")})
	require.NoError(t, err)
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		source any
		want   string
	}{
		{"non object", "csv text", "source must be an object"},
		{"empty source", map[string]any{}, "source is empty"},
		{
			"multiple forms",
			map[string]any{"file": "a.csv", "csv": "a\n1"},
			"source sets multiple forms",
		},
		{
			"columns without rows",
			map[string]any{"columns": []any{"a"}},
			"requires a parallel rows array",
		},
		{
			"bad columns type",
			map[string]any{"columns": "a", "rows": []any{}},
			"columns must be an array of strings",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load("", false, tc.source)
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

func TestLoadRowCap(t *testing.T) {
	t.Parallel()
	rows := make([][]any, DefaultMaxRows+1)
	for i := range rows {
		rows[i] = []any{i}
	}

	_, err := FromColumns([]string{"v"}, rows)
	assert.ErrorContains(t, err, "aggregate or sample the data")
}

func TestParseJSONForms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		data     string
		columns  []string
		firstRow []any
		wantErr  string
	}{
		{
			"object rows",
			`[{"month": "Jan", "revenue": 120}, {"month": "Feb"}]`,
			[]string{"month", "revenue"},
			[]any{"Jan", float64(120)},
			"",
		},
		{
			"named arrays",
			`{"revenue": [120, 145], "month": ["Jan", "Feb"]}`,
			[]string{"month", "revenue"},
			[]any{"Jan", float64(120)},
			"",
		},
		{
			"scalar rejected",
			`[1, 2]`,
			nil, nil, "json row 0 must be an object",
		},
		{
			"array length mismatch",
			`{"a": [1], "b": [1, 2]}`,
			nil, nil, "to match the other columns",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table, err := ParseJSON([]byte(tc.data))
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.columns, table.Columns)
			assert.Equal(t, tc.firstRow, table.Rows[0])
		})
	}
}

func TestLoadScopedFileFormats(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"data.csv":   "a,b\n1,x\n",
		"data.tsv":   "a\tb\n1\tx\n",
		"data.json":  `[{"a": 1, "b": "x"}]`,
		"data.jsonl": "{\"a\": 1, \"b\": \"x\"}\n{\"a\": 2, \"b\": \"y\"}\n",
	}
	for name, content := range files {
		t.Run(filepath.Ext(name), func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o600))

			table, err := Load(root, false, map[string]any{"file": name})
			require.NoError(t, err)
			assert.Equal(t, []string{"a", "b"}, table.Columns)
		})
	}
}

func TestLoadScopedFileUnsupported(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "data.txt"), []byte("x"), 0o600))

	_, err := Load(root, false, map[string]any{"file": "data.txt"})
	assert.ErrorContains(t, err, "unsupported source format")
}

func TestLoadScopedFileMissing(t *testing.T) {
	t.Parallel()

	_, err := Load(t.TempDir(), false, map[string]any{"file": "missing.csv"})
	assert.ErrorContains(t, err, "read file")
}
