package tools

import "pi-go/agent"

// CodingTools returns the default coding tool set (read, bash, edit, write).
func CodingTools(cwd string) []agent.AgentTool {
	return []agent.AgentTool{
		ReadTool(cwd),
		BashTool(cwd),
		EditTool(cwd),
		WriteTool(cwd),
	}
}

// ReadOnlyTools returns the read-only tool set (read, grep, find, ls).
func ReadOnlyTools(cwd string) []agent.AgentTool {
	return []agent.AgentTool{
		ReadTool(cwd),
		GrepTool(cwd),
		FindTool(cwd),
		LsTool(cwd),
	}
}

// AllTools returns all available tools.
func AllTools(cwd string) []agent.AgentTool {
	return []agent.AgentTool{
		ReadTool(cwd),
		BashTool(cwd),
		EditTool(cwd),
		WriteTool(cwd),
		GrepTool(cwd),
		FindTool(cwd),
		LsTool(cwd),
	}
}
