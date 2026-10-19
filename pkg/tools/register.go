package tools

import mcpserver "github.com/mark3labs/mcp-go/server"

// Register adds the render_chart tool to srv.
func Register(srv *mcpserver.MCPServer) error {
	srv.AddTool(newRenderChartTool(), handleRenderChart)
	return nil
}
