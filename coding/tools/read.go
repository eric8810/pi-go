// Package tools provides built-in tools for the coding agent.
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agentsdk/agent"
	"agentsdk/ai"
)

// ReadTool reads file contents with optional line range.
func ReadTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Tool: ai.Tool{
			Name:        "read",
			Description: "Read file contents. Returns the file content with line numbers.",
			Parameters: ai.ToolParameter{
				Type: "object",
				Properties: map[string]ai.ToolParameter{
					"path": {
						Type:        "string",
						Description: "The file path to read (relative to working directory or absolute)",
					},
					"offset": {
						Type:        "number",
						Description: "Line number to start reading from (1-based). Default: 1",
					},
					"limit": {
						Type:        "number",
						Description: "Maximum number of lines to read. Default: all lines",
					},
				},
				Required: []string{"path"},
			},
		},
		Label: "Read file",
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate agent.UpdateCallback) (*agent.ToolResult, error) {
			path := getString(params, "path")
			if path == "" {
				return agent.ErrorResult("path parameter is required"), nil
			}

			absPath := resolvePath(cwd, path)

			data, err := os.ReadFile(absPath)
			if err != nil {
				return agent.ErrorResult(fmt.Sprintf("Failed to read file: %v", err)), nil
			}

			lines := strings.Split(string(data), "\n")

			offset := getInt(params, "offset", 1)
			if offset < 1 {
				offset = 1
			}

			limit := getInt(params, "limit", 0)

			// Apply offset and limit
			startIdx := offset - 1
			if startIdx >= len(lines) {
				return agent.ErrorResult(fmt.Sprintf("Offset %d exceeds file length (%d lines)", offset, len(lines))), nil
			}

			endIdx := len(lines)
			if limit > 0 && startIdx+limit < endIdx {
				endIdx = startIdx + limit
			}

			selectedLines := lines[startIdx:endIdx]

			// Format with line numbers
			var sb strings.Builder
			for i, line := range selectedLines {
				lineNum := startIdx + i + 1
				fmt.Fprintf(&sb, "%6d\t%s\n", lineNum, line)
			}

			return agent.TextResult(sb.String()), nil
		},
	}
}

// resolvePath resolves a path relative to the working directory.
func resolvePath(cwd, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(cwd, path))
}

// getString extracts a string parameter.
func getString(params map[string]any, key string) string {
	v, ok := params[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}

// getInt extracts an integer parameter with a default value.
func getInt(params map[string]any, key string, defaultVal int) int {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return defaultVal
	}
}

// getBool extracts a boolean parameter with a default value.
func getBool(params map[string]any, key string, defaultVal bool) bool {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	b, ok := v.(bool)
	if !ok {
		return defaultVal
	}
	return b
}
