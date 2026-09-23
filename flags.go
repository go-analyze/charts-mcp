package main

import (
	"fmt"
	"os"

	"github.com/go-analyze/charts-mcp/pkg/config"
)

const usage = `charts-mcp — local MCP server for generating charts and graphs.

Usage:
  charts-mcp [flags]

Flags:
  -v, --version   print version and exit
  -h, --help      show this help and exit

The server speaks the Model Context Protocol over stdio.
`

// handleCLI reports whether a recognized CLI flag was handled
// and main should return instead of starting the server.
func handleCLI() bool {
	args := os.Args[1:]
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "-v", "--version":
		_, _ = fmt.Fprintf(os.Stdout, "charts-mcp version %s\n", config.Version)
	case "-h", "--help":
		_, _ = os.Stdout.WriteString(usage)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown flag: %s\n%s", args[0], usage)
	}
	return true
}
