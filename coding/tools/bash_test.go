package tools

import (
	"context"
	"strings"
	"testing"
)

func TestBashTool_SimpleCommand(t *testing.T) {
	dir := t.TempDir()

	tool := BashTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"command": "echo hello",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "hello") {
		t.Errorf("expected output to contain 'hello', got: %s", result.Content[0].Text)
	}
}

func TestBashTool_CapturesStdout(t *testing.T) {
	dir := t.TempDir()

	tool := BashTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"command": "printf 'stdout output'",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "stdout output") {
		t.Errorf("expected stdout output, got: %s", result.Content[0].Text)
	}
}

func TestBashTool_CapturesStderr(t *testing.T) {
	dir := t.TempDir()

	tool := BashTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"command": "echo 'stderr output' >&2",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// stderr with exit code 0 is not an error
	text := result.Content[0].Text
	if !strings.Contains(text, "STDERR:") {
		t.Errorf("expected STDERR label, got: %s", text)
	}
	if !strings.Contains(text, "stderr output") {
		t.Errorf("expected stderr content, got: %s", text)
	}
}

func TestBashTool_ExitCodeOnFailure(t *testing.T) {
	dir := t.TempDir()

	tool := BashTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"command": "exit 42",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for non-zero exit code")
	}
	if !strings.Contains(result.Content[0].Text, "Exit code: 42") {
		t.Errorf("expected exit code 42 in output, got: %s", result.Content[0].Text)
	}
}

func TestBashTool_Timeout(t *testing.T) {
	dir := t.TempDir()

	tool := BashTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		// Use a trap so the sleep ignores SIGTERM and the context deadline is hit cleanly
		"command": "trap '' TERM; sleep 30",
		"timeout": float64(1),
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for timed-out command")
	}
	// The implementation may return either "timed out" (context deadline) or
	// an exit code of -1 (signal kill). Both indicate the command did not
	// complete successfully within the timeout.
	text := result.Content[0].Text
	if !strings.Contains(text, "timed out") && !strings.Contains(text, "Exit code: -1") {
		t.Errorf("expected timeout or kill message, got: %s", text)
	}
}

func TestBashTool_RequiresCommand(t *testing.T) {
	dir := t.TempDir()

	tool := BashTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when command is missing")
	}
	if !strings.Contains(result.Content[0].Text, "command parameter is required") {
		t.Errorf("unexpected error message: %s", result.Content[0].Text)
	}
}
