package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pi-go/agent"
	"pi-go/ai"
)

// LsTool lists directory contents.
func LsTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Tool: ai.Tool{
			Name:        "ls",
			Description: "List directory contents with file sizes and types.",
			Parameters: ai.ToolParameter{
				Type: "object",
				Properties: map[string]ai.ToolParameter{
					"path": {
						Type:        "string",
						Description: "Directory path to list (relative to working directory). Default: current directory",
					},
					"recursive": {
						Type:        "boolean",
						Description: "If true, list contents recursively. Default: false",
					},
				},
			},
		},
		Label: "List directory",
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate agent.UpdateCallback) (*agent.ToolResult, error) {
			path := getString(params, "path")
			if path == "" {
				path = "."
			}
			recursive := getBool(params, "recursive", false)

			absPath := resolvePath(cwd, path)

			info, err := os.Stat(absPath)
			if err != nil {
				return agent.ErrorResult(fmt.Sprintf("Cannot access path: %v", err)), nil
			}
			if !info.IsDir() {
				return agent.ErrorResult(fmt.Sprintf("%s is not a directory", path)), nil
			}

			var results []string

			if recursive {
				err = filepath.Walk(absPath, func(p string, info os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					if ctx.Err() != nil {
						return ctx.Err()
					}

					// Skip hidden directories
					if info.IsDir() && info.Name() == ".git" {
						return filepath.SkipDir
					}

					relPath, _ := filepath.Rel(cwd, p)
					if relPath == "." {
						return nil
					}

					results = append(results, formatEntry(relPath, info))
					return nil
				})
			} else {
				entries, err := os.ReadDir(absPath)
				if err != nil {
					return agent.ErrorResult(fmt.Sprintf("Failed to read directory: %v", err)), nil
				}

				for _, entry := range entries {
					info, err := entry.Info()
					if err != nil {
						continue
					}
					relPath := filepath.Join(path, entry.Name())
					results = append(results, formatEntry(relPath, info))
				}
			}

			if err != nil && ctx.Err() == nil {
				return agent.ErrorResult(fmt.Sprintf("Error listing directory: %v", err)), nil
			}

			if len(results) == 0 {
				return agent.TextResult("(empty directory)"), nil
			}

			return agent.TextResult(strings.Join(results, "\n")), nil
		},
	}
}

// formatEntry formats a directory entry for display.
func formatEntry(path string, info os.FileInfo) string {
	if info.IsDir() {
		return fmt.Sprintf("  %s/", path)
	}
	size := formatSize(info.Size())
	return fmt.Sprintf("  %-50s %s", path, size)
}

// formatSize formats a file size in human-readable form.
func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1fG", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1fM", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1fK", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
