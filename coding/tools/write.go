package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"pi-go/agent"
	"pi-go/ai"
)

// WriteTool creates or overwrites files.
func WriteTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Tool: ai.Tool{
			Name:        "write",
			Description: "Create or overwrite a file with the given content.",
			Parameters: ai.ToolParameter{
				Type: "object",
				Properties: map[string]ai.ToolParameter{
					"path": {
						Type:        "string",
						Description: "The file path to write (relative to working directory or absolute)",
					},
					"content": {
						Type:        "string",
						Description: "The content to write to the file",
					},
				},
				Required: []string{"path", "content"},
			},
		},
		Label: "Write file",
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate agent.UpdateCallback) (*agent.ToolResult, error) {
			path := getString(params, "path")
			if path == "" {
				return agent.ErrorResult("path parameter is required"), nil
			}
			content := getString(params, "content")

			absPath := resolvePath(cwd, path)

			// Ensure parent directory exists
			dir := filepath.Dir(absPath)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return agent.ErrorResult(fmt.Sprintf("Failed to create directory: %v", err)), nil
			}

			if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
				return agent.ErrorResult(fmt.Sprintf("Failed to write file: %v", err)), nil
			}

			return agent.TextResult(fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path)), nil
		},
	}
}
