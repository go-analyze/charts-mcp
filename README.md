# go-analyze/charts-mcp

[![license](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/go-analyze/charts-mcp/blob/master/LICENSE)
[![Build Status](https://github.com/go-analyze/charts-mcp/actions/workflows/tests-main.yml/badge.svg)](https://github.com/go-analyze/charts-mcp/actions/workflows/tests-main.yml)

**MCP-based local chart generation tools for your agent**

Our server renders charts for AI agents on top of [go-analyze/charts](https://github.com/go-analyze/charts). The API is an ECharts option: the request body carries the supported subset of ECharts keys, and the server writes a rendered png, jpeg, or svg wherever you point it. Tabular data never has to cross the agent's context — series can bind to source columns instead of inlining rows.

## Getting Started

### 1. Install charts-mcp

```sh
go install github.com/go-analyze/charts-mcp@latest
```

### 2. Register the server with your MCP client

The server communicates over stdio, so point your client at the installed binary.

**Claude Code:**
```sh
claude mcp add charts -- charts-mcp
```

### 3. Render a chart

Chart a CSV on disk without reading it into context:

```json
{
  "source": { "file": "/path/to/data.csv" },
  "title": { "text": "Monthly revenue" },
  "theme": "dark",
  "width": 1200,
  "height": 700,
  "xAxis": { "column": "month" },
  "series": [
    { "type": "line", "field": "revenue", "name": "Revenue" }
  ],
  "output": { "path": "chart.png" }
}
```

Or fully inline with native ECharts data, no source at all:

```json
{
  "series": [
    { "type": "pie", "data": [{"value": 335, "name": "Direct"}, {"value": 310, "name": "Email"}] }
  ],
  "output": { "path": "slices.svg" }
}
```

## Functionality

The server exposes a single tool, `render_chart`, which renders an ECharts option and writes it to the output file. The full per-field contract lives in the tool schema; `docs/specs-v0.md` documents the request shape and `docs/echarts-supported.md` the renderer support matrix.

### Chart types

`line`, `bar`, `horizontalBar`, `scatter`, `pie`, `doughnut`, `radar`, `funnel`, `candlestick`, `violin`, and `horizontalViolin` via `series[].type`. Bar and violin flip horizontal when the category axis lands on y; unknown types error instead of rendering a blank chart.

### Data bindings

Series data arrives inline (`series[].data`, native ECharts shapes) or bound to a tabular source:

| Binding | Description |
| ------- | ----------- |
| `series[].field` | Numeric source column; missing cells render as gaps |
| `series[].columns` | Candlestick OHLC role-to-column map, assembled into `[open, close, lowest, highest]` tuples |
| `series[].source` | Per-series table overriding the shared source (multi-file charts) |
| implicit | A series with no data origin binds its table's sole numeric column silently (candlestick/violin excluded) |
| `xAxis.column` / `yAxis[].column` | Category label columns |

### Data Sources

`source` takes a file or inline data. Supported file formats are `csv`, `tsv`, `json`, and `jsonl`, with the extension deciding when omitted.

```json
{"file": "/path/to/data.csv"}
{"columns": ["month", "revenue"], "rows": [["Jan", 120], ["Feb", 145]]}
{"rows": [{"month": "Jan", "revenue": 120}]}
{"csv": "month,revenue\nJan,120\n"}
```

### Themes

Built-in palettes for the `theme` key: `light` (default), `dark`, `vivid-light`, `vivid-dark`, `grafana`, `ant`, `nature-light`, `nature-dark`, `retro`, `ocean`, `slate`, `gray`, `winter`, `spring`, `summer`, `fall`.

### Output

The rendered file is written to the required `output.path`; its extension (`.png`, `.jpg`, `.jpeg`, or `.svg`) selects the format and directories are created as needed. The tool result is text plus structured metadata (`width`, `height`, `format`, `bytes`, `warnings`) — image bytes are never returned inline. Chart dimensions are option keys (`width`/`height`, defaulting to 600x400).
