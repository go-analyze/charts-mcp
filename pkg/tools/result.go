package tools

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// structuredResult builds a tool result carrying metadata in both text (JSON on
// its own line) and structuredContent, so either client path works.
func structuredResult(text string, structured any) (*mcp.CallToolResult, error) {
	if encoded, err := json.Marshal(structured); err == nil {
		text += "\n" + string(encoded)
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{mcp.TextContent{Type: mcp.ContentTypeText, Text: text}},
		StructuredContent: structured,
	}, nil
}

// errorResult is the structuredContent contract of a failed render_chart call;
// the message mirrors the text content so either client path works.
type errorResult struct {
	Error string `json:"error"`
}

// toolError wraps a recoverable failure as an isError result carrying the
// message in both text and structuredContent; internal errors would surface to
// clients as unrecoverable server crashes.
func toolError(format string, args ...any) (*mcp.CallToolResult, error) {
	message := fmt.Sprintf(format, args...)
	return &mcp.CallToolResult{
		Content:           []mcp.Content{mcp.TextContent{Type: mcp.ContentTypeText, Text: message}},
		StructuredContent: errorResult{Error: message},
		IsError:           true,
	}, nil
}
