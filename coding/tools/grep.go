package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"pi-go/agent"
	"pi-go/ai"
)

const maxGrepResults = 200

// GrepTool searches file contents for patterns.
func GrepTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Tool: ai.Tool{
			Name:        "grep",
			Description: "Search file contents for a pattern (regex supported). Respects .gitignore patterns.",
			Parameters: ai.ToolParameter{
				Type: "object",
				Properties: map[string]ai.ToolParameter{
					"pattern": {
						Type:        "string",
						Description: "The regex pattern to search for",
					},
					"path": {
						Type:        "string",
						Description: "Directory or file to search in (relative to working directory). Default: current directory",
					},
					"include": {
						Type:        "string",
						Description: "Glob pattern to filter files (e.g., '*.go', '*.ts')",
					},
					"context_lines": {
						Type:        "number",
						Description: "Number of context lines before and after each match. Default: 0",
					},
				},
				Required: []string{"pattern"},
			},
		},
		Label: "Search files",
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate agent.UpdateCallback) (*agent.ToolResult, error) {
			pattern := getString(params, "pattern")
			if pattern == "" {
				return agent.ErrorResult("pattern parameter is required"), nil
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return agent.ErrorResult(fmt.Sprintf("Invalid regex pattern: %v", err)), nil
			}

			searchPath := getString(params, "path")
			if searchPath == "" {
				searchPath = "."
			}
			absPath := resolvePath(cwd, searchPath)

			include := getString(params, "include")
			contextLines := getInt(params, "context_lines", 0)

			var results []string
			count := 0

			err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil // skip errors
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if count >= maxGrepResults {
					return filepath.SkipAll
				}

				// Skip directories
				if info.IsDir() {
					name := info.Name()
					if name == ".git" || name == "node_modules" || name == ".venv" || name == "__pycache__" {
						return filepath.SkipDir
					}
					return nil
				}

				// Skip binary and large files
				if info.Size() > 1024*1024 { // 1MB limit
					return nil
				}

				// Apply include filter
				if include != "" {
					matched, _ := filepath.Match(include, info.Name())
					if !matched {
						return nil
					}
				}

				// Search file
				matches := searchFile(path, re, contextLines)
				if len(matches) > 0 {
					relPath, _ := filepath.Rel(cwd, path)
					if relPath == "" {
						relPath = path
					}
					for _, m := range matches {
						if count >= maxGrepResults {
							break
						}
						results = append(results, fmt.Sprintf("%s:%s", relPath, m))
						count++
					}
				}

				return nil
			})

			if err != nil && ctx.Err() == nil {
				return agent.ErrorResult(fmt.Sprintf("Search error: %v", err)), nil
			}

			if len(results) == 0 {
				return agent.TextResult("No matches found."), nil
			}

			output := strings.Join(results, "\n")
			if count >= maxGrepResults {
				output += fmt.Sprintf("\n\n... (results truncated at %d matches)", maxGrepResults)
			}

			return agent.TextResult(output), nil
		},
	}
}

// searchFile searches a single file for regex matches.
func searchFile(path string, re *regexp.Regexp, contextLines int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	var results []string
	for i, line := range allLines {
		if re.MatchString(line) {
			if contextLines > 0 {
				start := i - contextLines
				if start < 0 {
					start = 0
				}
				end := i + contextLines + 1
				if end > len(allLines) {
					end = len(allLines)
				}
				var contextBlock strings.Builder
				for j := start; j < end; j++ {
					prefix := " "
					if j == i {
						prefix = ">"
					}
					fmt.Fprintf(&contextBlock, "%s%d:%s\n", prefix, j+1, allLines[j])
				}
				results = append(results, "\n"+contextBlock.String())
			} else {
				results = append(results, fmt.Sprintf("%d:%s", i+1, line))
			}
		}
	}

	return results
}
