package source

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-analyze/bulk"
)

// DefaultMaxRows caps loaded sources so oversized inputs fail fast instead of
// being dumped into memory.
const DefaultMaxRows = 100_000

// maxJSONLLineBytes bounds a single jsonl line for the scanner. The scanner
// grows its buffer lazily up to this cap.
const maxJSONLLineBytes = 64 << 20

// Load builds a Table from one of the four shared tool source forms: a file
// path, inline CSV text, columns plus array rows, or an array of object rows.
// Relative file paths resolve against the working directory.
func Load(value any) (*Table, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("source must be an object with one of: file, csv, columns+rows, rows")
	}
	_, hasFile := object["file"]
	_, hasCSV := object["csv"]
	_, hasColumns := object["columns"]
	_, hasRows := object["rows"]

	switch {
	case hasFile && (hasCSV || hasColumns || hasRows), hasCSV && (hasColumns || hasRows):
		return nil, errors.New("source sets multiple forms; use exactly one of file, csv, columns+rows, or rows")
	case hasFile:
		path, ok := object["file"].(string)
		if !ok {
			return nil, errors.New("source file must be a string path")
		}
		return LoadFile(path)
	case hasCSV:
		text, ok := object["csv"].(string)
		if !ok {
			return nil, errors.New("source csv must be text")
		}
		return ParseCSV(text)
	case hasColumns:
		if !hasRows {
			return nil, errors.New("source columns requires a parallel rows array")
		}
		columns, err := sourceColumns(object["columns"])
		if err != nil {
			return nil, err
		}
		rows, err := sourceArrayRows(object["rows"])
		if err != nil {
			return nil, err
		}
		return FromColumns(columns, rows)
	case hasRows:
		rows, err := sourceObjectRows(object["rows"])
		if err != nil {
			return nil, err
		}
		return FromObjectRows(rows)
	default:
		return nil, errors.New("source is empty; use one of: file, csv, columns+rows, rows")
	}
}

// sourceColumns converts a decoded JSON string array into column names.
func sourceColumns(value any) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, errors.New("source columns must be an array of strings")
	}
	columns := make([]string, len(items))
	for i, item := range items {
		name, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("source column %d must be a string", i)
		}
		columns[i] = name
	}
	return columns, nil
}

// sourceArrayRows converts decoded JSON arrays into array rows.
func sourceArrayRows(value any) ([][]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, errors.New("source rows must be an array of row arrays")
	}
	rows := make([][]any, len(items))
	for i, item := range items {
		row, ok := item.([]any)
		if !ok {
			return nil, fmt.Errorf("source row %d must be an array to pair with columns", i)
		}
		rows[i] = row
	}
	return rows, nil
}

// sourceObjectRows converts decoded JSON objects into object rows.
func sourceObjectRows(value any) ([]map[string]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, errors.New("source rows must be an array of objects")
	}
	rows := make([]map[string]any, len(items))
	for i, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			if _, isArray := item.([]any); isArray {
				return nil, fmt.Errorf(
					"source row %d is an array; supply columns to plot array rows", i)
			}
			return nil, fmt.Errorf("source row %d must be an object; key rows by column name", i)
		}
		rows[i] = row
	}
	return rows, nil
}

func newTable(columns []string, rows [][]any) (*Table, error) {
	if len(rows) > DefaultMaxRows {
		return nil, fmt.Errorf("source has %d rows; limit is %d; aggregate or sample the data",
			len(rows), DefaultMaxRows)
	}
	if set := bulk.SliceToSet(columns); len(set) != len(columns) {
		return nil, errors.New("duplicate column names in source")
	}
	return &Table{Columns: columns, Rows: rows}, nil
}

// ParseCSV loads a table from CSV text with a header row. Cells stay strings;
// typed access goes through the As* helpers.
func ParseCSV(text string) (*Table, error) {
	return parseDelimited(text, ',')
}

// ParseTSV loads a table from TSV text with a header row, like ParseCSV.
func ParseTSV(text string) (*Table, error) {
	return parseDelimited(text, '\t')
}

func parseDelimited(text string, comma rune) (*Table, error) {
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = comma
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse delimited data: %w", err)
	} else if len(records) == 0 {
		return nil, errors.New("empty source; expected a header row")
	}
	rows := make([][]any, 0, len(records)-1)
	for _, record := range records[1:] {
		cells := make([]any, len(record))
		for i, cell := range record {
			cells[i] = cell
		}
		rows = append(rows, cells)
	}
	return newTable(records[0], rows)
}

// ReadFile returns the contents and resolved path of a file. Used for option
// files and other raw-content reads.
func ReadFile(path string) ([]byte, string, error) {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return nil, "", fmt.Errorf("resolve path: %w", err)
	}
	data, err := os.ReadFile(resolved) //nolint:gosec // unrestricted read paths are intended
	if err != nil {
		return nil, "", fmt.Errorf("read file: %w", err)
	}
	return data, resolved, nil
}

// LoadFile reads a data file, inferring the format from its extension: csv,
// tsv, json, or jsonl.
func LoadFile(path string) (*Table, error) {
	data, resolved, err := ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch ext := strings.ToLower(filepath.Ext(resolved)); ext {
	case ".csv":
		return ParseCSV(string(data))
	case ".tsv":
		return ParseTSV(string(data))
	case ".json":
		return ParseJSON(data)
	case ".jsonl":
		return parseJSONL(data)
	default:
		return nil, fmt.Errorf("unsupported source format %q; use csv, tsv, json, or jsonl", ext)
	}
}

// ParseJSON loads a flat array of objects or a single object with named arrays.
func ParseJSON(data []byte) (*Table, error) {
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("parse json source: %w", err)
	}
	switch value := decoded.(type) {
	case []any:
		rows := make([]map[string]any, 0, len(value))
		for i, item := range value {
			object, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf(
					"json row %d must be an object; key rows by column name, or use the csv/columns+rows source forms", i)
			}
			rows = append(rows, object)
		}
		return FromObjectRows(rows)
	case map[string]any:
		return fromNamedArrays(value)
	default:
		return nil, errors.New("json source must be an array of objects or a single object with named arrays")
	}
}

func parseJSONL(data []byte) (*Table, error) {
	var rows []map[string]any
	lineNumber := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), maxJSONLLineBytes)
	for scanner.Scan() {
		lineNumber++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(text), &row); err != nil {
			return nil, fmt.Errorf("parse jsonl line %d (each line must be an object): %w", lineNumber, err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read jsonl source: %w", err)
	}
	return FromObjectRows(rows)
}

// FromColumns builds a table from column names plus array rows.
func FromColumns(columns []string, rows [][]any) (*Table, error) {
	return newTable(slices.Clone(columns), rows)
}

// FromObjectRows builds a table from objects keyed by column name. Column order
// follows first appearance; missing keys read as nil.
func FromObjectRows(rows []map[string]any) (*Table, error) {
	columns := make([]string, 0)
	indexOf := make(map[string]int)
	for _, row := range rows {
		keys := bulk.MapKeysSlice(row)
		slices.Sort(keys)
		for _, key := range keys {
			if _, ok := indexOf[key]; !ok {
				indexOf[key] = len(columns)
				columns = append(columns, key)
			}
		}
	}
	table := make([][]any, len(rows))
	for i, row := range rows {
		cells := make([]any, len(columns))
		for j, column := range columns {
			cells[j] = row[column]
		}
		table[i] = cells
	}
	return newTable(columns, table)
}

// fromNamedArrays builds a table from an object of named arrays. Column order is
// the sorted key set since JSON decoding loses insertion order.
func fromNamedArrays(object map[string]any) (*Table, error) {
	if len(object) == 0 {
		return newTable(nil, nil)
	}
	columns := bulk.MapKeysSlice(object)
	slices.Sort(columns)

	length := -1
	arrays := make(map[string][]any, len(columns))
	for _, column := range columns {
		array, ok := object[column].([]any)
		if !ok {
			return nil, fmt.Errorf("column %q must map to an array of values", column)
		} else if length == -1 {
			length = len(array)
		} else if len(array) != length {
			return nil, fmt.Errorf("column %q has %d values; expected %d to match the other columns",
				column, len(array), length)
		}
		arrays[column] = array
	}

	rows := make([][]any, max(length, 0))
	for i := range rows {
		rows[i] = make([]any, len(columns))
	}
	for j, column := range columns {
		for i, value := range arrays[column] {
			rows[i][j] = value
		}
	}
	return newTable(columns, rows)
}
