# go-analyze/charts-mcp

[![license](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/go-analyze/charts-mcp/blob/master/LICENSE)
[![Build Status](https://github.com/go-analyze/charts-mcp/actions/workflows/tests-main.yml/badge.svg)](https://github.com/go-analyze/charts-mcp/actions/workflows/tests-main.yml)

**Local chart rendering for your AI agents via MCP**

charts-mcp is a stdio MCP server built on top of our Go charting library, [go-analyze/charts](https://github.com/go-analyze/charts). Your agent describes the chart it wants and names an output file, and the server writes a rendered png, jpeg, or svg on your own machine.

## Getting Started

### 1. Install charts-mcp

```sh
go install github.com/go-analyze/charts-mcp@latest
```

The binary is installed into `$(go env GOPATH)/bin`. Keep that directory on your `PATH` so your MCP client can launch it.

### 2. Register the server with your MCP client

charts-mcp speaks MCP over stdio. It takes no arguments, no flags, and no configuration file, so you only need to add it as a local stdio server in any client that can spawn a process. For Claude Code:

```sh
claude mcp add charts -- charts-mcp
```

Every other client needs the same two values: the command `charts-mcp` and a stdio transport. Restart the client afterwards so it picks up the new tool, then ask your agent for something like "chart monthly revenue from sales.csv as a line chart and save it to revenue.png". The agent calls `render_chart` and tells you where the file landed.

## Features

* **Runs locally:** Rendering happens inside the server process with no browser, no Node runtime, and no network calls. Your data and your charts never leave the machine, and there is no third-party service or API key to configure.
* **Fast:** A chart is drawn directly by Go rather than by a headless browser, so there is no page load to pay for on each request. A render completes in milliseconds, which keeps an agent's render and review loop cheap enough to iterate on.
* **Matches our [charts](https://github.com/go-analyze/charts) styling:** Every request is drawn by the same renderer and the same built-in [themes](https://github.com/go-analyze/charts/wiki/Feature-Overview#themes) as our Go charting library, so output matches charts rendered directly in Go.
* **Full ECharts for agents:** Styling, layout, axes, legends, marks, and themes are all expressed with native ECharts option keys, covering as much of ECharts as our renderer supports today.
* **Keeps your data out of context:** Series can bind to columns of a tabular source instead of carrying rows inline, so an agent can chart a large csv or json file on disk without spending tokens reading it in and sending its rows back through the MCP API. Row count does not affect the size of the request.

The per-field contract lives in the `render_chart` tool schema, which is what your agent reads while it works.

## API Stability

Although our chart library is very stable, this agent API is new. We are exploring the best way for agents to configure and generate charts. We do not recommend depending on the MCP API externally as it may change without notice.

If you witness agent usage of the MCP API that you think could be improved, please open an issue so it can be considered.
