package tools

import mcpserver "github.com/mark3labs/mcp-go/server"

// Register is the extension point for adding inspect_data and render_chart
// to a server.
func Register(srv *mcpserver.MCPServer) error {
	_ = srv
	return nil
}
