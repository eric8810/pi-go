package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"pi-go/agent"
	"pi-go/ai"
)

// EditTool performs surgical find-and-replace edits on files.
func EditTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Tool: ai.Tool{
			Name:        "edit",
			Description: "Make surgical edits to a file by finding exact text and replacing it. The old_text must match exactly (including whitespace and indentation).",
			Parameters: ai.ToolParameter{
				Type: "object",
				Properties: map[string]ai.ToolParameter{
					"path": {
						Type:        "string",
						Description: "The file path to edit (relative to working directory or absolute)",
					},
					"old_text": {
						Type:        "string",
						Description: "The exact text to find in the file",
					},
					"new_text": {
						Type:        "string",
						Description: "The text to replace old_text with",
					},
					"replace_all": {
						Type:        "boolean",
						Description: "If true, replace all occurrences. Default: false (only first occurrence)",
					},
				},
				Required: []string{"path", "old_text", "new_text"},
			},
		},
		Label: "Edit file",
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate agent.UpdateCallback) (*agent.ToolResult, error) {
			path := getString(params, "path")
			if path == "" {
				return agent.ErrorResult("path parameter is required"), nil
			}
			oldText := getString(params, "old_text")
			newText := getString(params, "new_text")
			replaceAll := getBool(params, "replace_all", false)

			if oldText == "" {
				return agent.ErrorResult("old_text parameter is required"), nil
			}

			absPath := resolvePath(cwd, path)

			data, err := os.ReadFile(absPath)
			if err != nil {
				return agent.ErrorResult(fmt.Sprintf("Failed to read file: %v", err)), nil
			}

			content := string(data)

			// Check that old_text exists
			count := strings.Count(content, oldText)
			if count == 0 {
				return agent.ErrorResult("old_text not found in file. Make sure it matches exactly (including whitespace and indentation)."), nil
			}

			if !replaceAll && count > 1 {
				return agent.ErrorResult(fmt.Sprintf("old_text found %d times in file. Either provide more context to make the match unique, or set replace_all to true.", count)), nil
			}

			// Perform replacement
			var newContent string
			if replaceAll {
				newContent = strings.ReplaceAll(content, oldText, newText)
			} else {
				newContent = strings.Replace(content, oldText, newText, 1)
			}

			if err := os.WriteFile(absPath, []byte(newContent), 0o644); err != nil {
				return agent.ErrorResult(fmt.Sprintf("Failed to write file: %v", err)), nil
			}

			replacements := 1
			if replaceAll {
				replacements = count
			}

			return agent.TextResult(fmt.Sprintf("Successfully edited %s (%d replacement(s))", path, replacements)), nil
		},
	}
}
