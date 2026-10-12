package main

import (
	"fmt"
	"os"

	"github.com/go-analyze/charts-mcp/pkg/config"
	"github.com/go-analyze/charts-mcp/pkg/tools"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func main() {
	if handleCLI() {
		return
	}

	config.LoadEnv()
	srv := mcpserver.NewMCPServer("charts", config.Version)

	if err := tools.Register(srv); err != nil {
		fmt.Fprintln(os.Stderr, "register tools:", err)
		os.Exit(1)
	}

	if err := mcpserver.ServeStdio(srv); err != nil {
		fmt.Fprintln(os.Stderr, "serve stdio:", err)
		os.Exit(1)
	}
}
