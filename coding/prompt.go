package coding

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// SystemPromptOptions configures system prompt generation.
type SystemPromptOptions struct {
	// CustomPrompt replaces the default system prompt entirely.
	CustomPrompt string
	// AppendPrompt is appended to the system prompt.
	AppendPrompt string
	// Cwd is the working directory. Default: os.Getwd()
	Cwd string
	// SelectedTools lists the tools to describe in the prompt.
	SelectedTools []string
	// ContextFiles are project-specific context files to include.
	ContextFiles []ContextFile
}

// ContextFile is a project context file included in the system prompt.
type ContextFile struct {
	Path    string
	Content string
}

// Tool descriptions for the system prompt.
var toolDescriptions = map[string]string{
	"read":  "Read file contents with line numbers",
	"bash":  "Execute bash commands (ls, grep, find, etc.)",
	"edit":  "Make surgical edits to files (find exact text and replace)",
	"write": "Create or overwrite files",
	"grep":  "Search file contents for patterns (respects .gitignore)",
	"find":  "Find files by glob pattern (respects .gitignore)",
	"ls":    "List directory contents",
}

// BuildSystemPrompt constructs the system prompt with tools, guidelines, and context.
func BuildSystemPrompt(opts SystemPromptOptions) string {
	cwd := opts.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	now := time.Now()
	dateTime := now.Format("Monday, January 2, 2006 3:04:05 PM MST")

	if opts.CustomPrompt != "" {
		prompt := opts.CustomPrompt
		if opts.AppendPrompt != "" {
			prompt += "\n\n" + opts.AppendPrompt
		}
		if len(opts.ContextFiles) > 0 {
			prompt += "\n\n# Project Context\n\nProject-specific instructions and guidelines:\n\n"
			for _, cf := range opts.ContextFiles {
				prompt += fmt.Sprintf("## %s\n\n%s\n\n", cf.Path, cf.Content)
			}
		}
		return prompt
	}

	var sb strings.Builder

	// Header
	sb.WriteString("You are an expert software engineer assistant. ")
	sb.WriteString("You help users with coding tasks by reading, writing, and modifying code.\n\n")

	// Environment info
	sb.WriteString("# Environment\n\n")
	sb.WriteString(fmt.Sprintf("- Date: %s\n", dateTime))
	sb.WriteString(fmt.Sprintf("- Working directory: %s\n", cwd))
	sb.WriteString(fmt.Sprintf("- OS: %s/%s\n", runtime.GOOS, runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("- Shell: %s\n", getShell()))
	sb.WriteString("\n")

	// Available tools
	selectedTools := opts.SelectedTools
	if len(selectedTools) == 0 {
		selectedTools = []string{"read", "bash", "edit", "write"}
	}

	sb.WriteString("# Available Tools\n\n")
	for _, tool := range selectedTools {
		desc, ok := toolDescriptions[tool]
		if ok {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", tool, desc))
		}
	}
	sb.WriteString("\n")

	// Guidelines
	sb.WriteString("# Guidelines\n\n")
	sb.WriteString("1. Always read a file before editing it to understand existing code.\n")
	sb.WriteString("2. Use the edit tool for surgical changes. Use write only for new files or complete rewrites.\n")
	sb.WriteString("3. When editing, provide enough context in old_text to uniquely identify the location.\n")
	sb.WriteString("4. Run tests and verify your changes work before completing a task.\n")
	sb.WriteString("5. Keep responses concise and focused on the task.\n")
	sb.WriteString("6. If you're unsure about something, ask the user rather than guessing.\n")
	sb.WriteString("7. Be careful with destructive operations (deleting files, force-pushing, etc.).\n")
	sb.WriteString("8. Do not introduce security vulnerabilities (command injection, XSS, SQL injection, etc.).\n")
	sb.WriteString("\n")

	// Project context
	if len(opts.ContextFiles) > 0 {
		sb.WriteString("# Project Context\n\n")
		sb.WriteString("Project-specific instructions and guidelines:\n\n")
		for _, cf := range opts.ContextFiles {
			sb.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", cf.Path, cf.Content))
		}
	}

	// Load project-level context files
	projectContext := loadProjectContext(cwd)
	if projectContext != "" {
		sb.WriteString(projectContext)
	}

	// Append custom prompt
	if opts.AppendPrompt != "" {
		sb.WriteString("\n\n")
		sb.WriteString(opts.AppendPrompt)
	}

	return sb.String()
}

// loadProjectContext looks for project context files (AGENT.md, .pi/context.md, etc.)
func loadProjectContext(cwd string) string {
	contextFiles := []string{
		"AGENT.md",
		".pi/context.md",
		".pi/instructions.md",
	}

	var sb strings.Builder
	for _, name := range contextFiles {
		path := filepath.Join(cwd, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", name, string(data)))
	}

	if sb.Len() > 0 {
		return "# Project Files\n\n" + sb.String()
	}
	return ""
}

// getShell returns the current shell.
func getShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		if runtime.GOOS == "windows" {
			return "cmd"
		}
		return "/bin/sh"
	}
	return filepath.Base(shell)
}
