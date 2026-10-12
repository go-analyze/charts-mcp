package tools

import mcpserver "github.com/mark3labs/mcp-go/server"

// Register adds the inspect_data and render_chart tools to srv.
func Register(srv *mcpserver.MCPServer) error {
	srv.AddTool(newInspectDataTool(), handleInspectData)
	srv.AddTool(newRenderChartTool(), handleRenderChart)
	return nil
}
