package tools

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// callRequest builds a CallToolRequest carrying decoded JSON arguments.
func callRequest(t *testing.T, tool string, arguments map[string]any) mcp.CallToolRequest {
	t.Helper()

	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: tool, Arguments: arguments},
	}
}

// resultText joins text content blocks of a result.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	require.NotNil(t, result)
	var parts []string
	for _, content := range result.Content {
		if block, ok := content.(mcp.TextContent); ok {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// requireErrorResult asserts the recoverable error contract: isError set with a
// message in both text and the structured error envelope, and no transport-level
// failure.
func requireErrorResult(t *testing.T, result *mcp.CallToolResult, err error, contains string) {
	t.Helper()

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	envelope, ok := result.StructuredContent.(errorResult)
	require.True(t, ok)
	assert.NotEmpty(t, envelope.Error)
	if contains != "" {
		text := resultText(t, result)
		assert.Contains(t, text, contains)
		assert.Equal(t, text, envelope.Error)
	}
}

// hasImageBlock reports whether any content block carries an inline image.
func hasImageBlock(result *mcp.CallToolResult) bool {
	for _, content := range result.Content {
		if _, ok := content.(mcp.ImageContent); ok {
			return true
		}
	}
	return false
}
