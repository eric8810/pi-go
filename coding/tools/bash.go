package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"agentsdk/agent"
	"agentsdk/ai"
)

const (
	defaultBashTimeout = 120 * time.Second
	maxOutputBytes     = 100 * 1024 // 100KB output limit
)

// BashTool executes bash commands.
func BashTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Tool: ai.Tool{
			Name:        "bash",
			Description: "Execute a bash command. The command runs in a shell with the working directory set to the project root.",
			Parameters: ai.ToolParameter{
				Type: "object",
				Properties: map[string]ai.ToolParameter{
					"command": {
						Type:        "string",
						Description: "The bash command to execute",
					},
					"timeout": {
						Type:        "number",
						Description: "Timeout in seconds. Default: 120",
					},
				},
				Required: []string{"command"},
			},
		},
		Label: "Execute bash",
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate agent.UpdateCallback) (*agent.ToolResult, error) {
			command := getString(params, "command")
			if command == "" {
				return agent.ErrorResult("command parameter is required"), nil
			}

			timeout := defaultBashTimeout
			if t := getInt(params, "timeout", 0); t > 0 {
				timeout = time.Duration(t) * time.Second
			}

			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			cmd := exec.CommandContext(ctx, "bash", "-c", command)
			cmd.Dir = cwd

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			// Stream output updates
			err := cmd.Run()

			var sb strings.Builder

			if stdout.Len() > 0 {
				output := stdout.String()
				if len(output) > maxOutputBytes {
					output = output[:maxOutputBytes] + "\n... (output truncated)"
				}
				sb.WriteString(output)
			}

			if stderr.Len() > 0 {
				errOutput := stderr.String()
				if len(errOutput) > maxOutputBytes {
					errOutput = errOutput[:maxOutputBytes] + "\n... (output truncated)"
				}
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString("STDERR:\n")
				sb.WriteString(errOutput)
			}

			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else if ctx.Err() != nil {
					return agent.ErrorResult(fmt.Sprintf("Command timed out after %v", timeout)), nil
				} else {
					return agent.ErrorResult(fmt.Sprintf("Failed to execute command: %v", err)), nil
				}
			}

			result := sb.String()
			if result == "" {
				result = "(no output)"
			}

			if exitCode != 0 {
				result = fmt.Sprintf("Exit code: %d\n%s", exitCode, result)
				return &agent.ToolResult{
					Content: []ai.ContentBlock{ai.TextBlock(result)},
					IsError: true,
				}, nil
			}

			return agent.TextResult(result), nil
		},
	}
}
