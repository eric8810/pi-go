package coding

import (
	"runtime"
	"strings"
	"testing"
)

func TestBuildSystemPrompt_Defaults(t *testing.T) {
	dir := t.TempDir()
	prompt := BuildSystemPrompt(SystemPromptOptions{
		Cwd: dir,
	})

	// Should contain environment info
	if !strings.Contains(prompt, "Environment") {
		t.Error("expected prompt to contain 'Environment'")
	}
	if !strings.Contains(prompt, dir) {
		t.Errorf("expected prompt to contain working directory %q", dir)
	}
	osArch := runtime.GOOS + "/" + runtime.GOARCH
	if !strings.Contains(prompt, osArch) {
		t.Errorf("expected prompt to contain OS/arch %q", osArch)
	}

	// Should contain default tools
	if !strings.Contains(prompt, "read") {
		t.Error("expected prompt to mention 'read' tool")
	}
	if !strings.Contains(prompt, "bash") {
		t.Error("expected prompt to mention 'bash' tool")
	}
	if !strings.Contains(prompt, "edit") {
		t.Error("expected prompt to mention 'edit' tool")
	}
	if !strings.Contains(prompt, "write") {
		t.Error("expected prompt to mention 'write' tool")
	}

	// Should contain guidelines
	if !strings.Contains(prompt, "Guidelines") {
		t.Error("expected prompt to contain 'Guidelines'")
	}
}

func TestBuildSystemPrompt_CustomPrompt(t *testing.T) {
	prompt := BuildSystemPrompt(SystemPromptOptions{
		CustomPrompt: "You are a custom assistant.",
		Cwd:          t.TempDir(),
	})

	if !strings.Contains(prompt, "You are a custom assistant.") {
		t.Error("expected prompt to contain custom prompt text")
	}
	// Should NOT contain default sections
	if strings.Contains(prompt, "# Environment") {
		t.Error("custom prompt should not contain default Environment section")
	}
	if strings.Contains(prompt, "# Guidelines") {
		t.Error("custom prompt should not contain default Guidelines section")
	}
}

func TestBuildSystemPrompt_AppendPrompt(t *testing.T) {
	prompt := BuildSystemPrompt(SystemPromptOptions{
		AppendPrompt: "Always respond in haiku.",
		Cwd:          t.TempDir(),
	})

	if !strings.Contains(prompt, "Always respond in haiku.") {
		t.Error("expected prompt to contain appended text")
	}
	// Should still have the default sections
	if !strings.Contains(prompt, "# Environment") {
		t.Error("expected default Environment section with append prompt")
	}
}

func TestBuildSystemPrompt_CustomWithAppend(t *testing.T) {
	prompt := BuildSystemPrompt(SystemPromptOptions{
		CustomPrompt: "Custom base.",
		AppendPrompt: "Extra instructions.",
		Cwd:          t.TempDir(),
	})

	if !strings.Contains(prompt, "Custom base.") {
		t.Error("expected custom prompt text")
	}
	if !strings.Contains(prompt, "Extra instructions.") {
		t.Error("expected appended text")
	}
}

func TestBuildSystemPrompt_SelectedTools(t *testing.T) {
	prompt := BuildSystemPrompt(SystemPromptOptions{
		SelectedTools: []string{"read", "grep"},
		Cwd:           t.TempDir(),
	})

	if !strings.Contains(prompt, "**read**") {
		t.Error("expected prompt to describe 'read' tool")
	}
	if !strings.Contains(prompt, "**grep**") {
		t.Error("expected prompt to describe 'grep' tool")
	}
	// Should NOT contain tools not in the selected set
	if strings.Contains(prompt, "**bash**") {
		t.Error("prompt should not describe 'bash' tool when not selected")
	}
	if strings.Contains(prompt, "**edit**") {
		t.Error("prompt should not describe 'edit' tool when not selected")
	}
}

func TestBuildSystemPrompt_ContextFiles(t *testing.T) {
	prompt := BuildSystemPrompt(SystemPromptOptions{
		Cwd: t.TempDir(),
		ContextFiles: []ContextFile{
			{Path: "AGENT.md", Content: "This project uses Go."},
			{Path: ".pi/instructions.md", Content: "Use tabs for indentation."},
		},
	})

	if !strings.Contains(prompt, "Project Context") {
		t.Error("expected 'Project Context' section")
	}
	if !strings.Contains(prompt, "AGENT.md") {
		t.Error("expected context file path 'AGENT.md'")
	}
	if !strings.Contains(prompt, "This project uses Go.") {
		t.Error("expected context file content")
	}
	if !strings.Contains(prompt, ".pi/instructions.md") {
		t.Error("expected context file path '.pi/instructions.md'")
	}
	if !strings.Contains(prompt, "Use tabs for indentation.") {
		t.Error("expected context file content")
	}
}

func TestBuildSystemPrompt_CustomPromptWithContextFiles(t *testing.T) {
	prompt := BuildSystemPrompt(SystemPromptOptions{
		CustomPrompt: "Custom prompt only.",
		Cwd:          t.TempDir(),
		ContextFiles: []ContextFile{
			{Path: "context.md", Content: "Some context."},
		},
	})

	if !strings.Contains(prompt, "Custom prompt only.") {
		t.Error("expected custom prompt text")
	}
	if !strings.Contains(prompt, "Project Context") {
		t.Error("expected Project Context section even with custom prompt")
	}
	if !strings.Contains(prompt, "Some context.") {
		t.Error("expected context file content")
	}
}
