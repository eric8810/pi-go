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

const maxFindResults = 500

// FindTool finds files by glob pattern.
func FindTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Tool: ai.Tool{
			Name:        "find",
			Description: "Find files by glob pattern. Respects .gitignore patterns. Returns matching file paths.",
			Parameters: ai.ToolParameter{
				Type: "object",
				Properties: map[string]ai.ToolParameter{
					"pattern": {
						Type:        "string",
						Description: "Glob pattern to match files (e.g., '**/*.go', 'src/**/*.ts')",
					},
					"path": {
						Type:        "string",
						Description: "Directory to search in (relative to working directory). Default: current directory",
					},
				},
				Required: []string{"pattern"},
			},
		},
		Label: "Find files",
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate agent.UpdateCallback) (*agent.ToolResult, error) {
			pattern := getString(params, "pattern")
			if pattern == "" {
				return agent.ErrorResult("pattern parameter is required"), nil
			}

			searchPath := getString(params, "path")
			if searchPath == "" {
				searchPath = "."
			}
			absPath := resolvePath(cwd, searchPath)

			var results []string
			count := 0

			err := filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if count >= maxFindResults {
					return filepath.SkipAll
				}

				// Skip hidden directories
				if info.IsDir() {
					name := info.Name()
					if name == ".git" || name == "node_modules" || name == ".venv" || name == "__pycache__" {
						return filepath.SkipDir
					}
					return nil
				}

				relPath, _ := filepath.Rel(cwd, path)
				if relPath == "" {
					relPath = path
				}

				// Match against pattern
				matched, _ := filepath.Match(pattern, info.Name())
				if !matched {
					// Try matching against full relative path
					matched, _ = filepath.Match(pattern, relPath)
				}
				if !matched {
					// Handle ** patterns by matching just the filename against the base pattern
					if strings.Contains(pattern, "**") {
						base := filepath.Base(pattern)
						matched, _ = filepath.Match(base, info.Name())
					}
				}

				if matched {
					results = append(results, relPath)
					count++
				}

				return nil
			})

			if err != nil && ctx.Err() == nil {
				return agent.ErrorResult(fmt.Sprintf("Search error: %v", err)), nil
			}

			if len(results) == 0 {
				return agent.TextResult("No files found matching pattern."), nil
			}

			output := strings.Join(results, "\n")
			if count >= maxFindResults {
				output += fmt.Sprintf("\n\n... (results truncated at %d files)", maxFindResults)
			}

			return agent.TextResult(output), nil
		},
	}
}
