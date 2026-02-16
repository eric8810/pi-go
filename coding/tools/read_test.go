package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTool_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := ReadTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path": "hello.txt",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	// Should contain line numbers
	if !strings.Contains(text, "1\tline1") {
		t.Errorf("expected line 1 with content, got:\n%s", text)
	}
	if !strings.Contains(text, "2\tline2") {
		t.Errorf("expected line 2 with content, got:\n%s", text)
	}
	if !strings.Contains(text, "3\tline3") {
		t.Errorf("expected line 3 with content, got:\n%s", text)
	}
}

func TestReadTool_NonExistentFile(t *testing.T) {
	dir := t.TempDir()

	tool := ReadTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path": "nonexistent.txt",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for non-existent file")
	}
	if !strings.Contains(result.Content[0].Text, "Failed to read file") {
		t.Errorf("unexpected error message: %s", result.Content[0].Text)
	}
}

func TestReadTool_WithOffset(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "lines.txt")
	if err := os.WriteFile(filePath, []byte("a\nb\nc\nd\ne\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := ReadTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path":   "lines.txt",
		"offset": float64(3),
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	// Line 3 should be "c"
	if !strings.Contains(text, "3\tc") {
		t.Errorf("expected line 3 to contain 'c', got:\n%s", text)
	}
	// Line 1 and 2 should NOT be present
	if strings.Contains(text, "1\ta") {
		t.Errorf("line 1 should not be present, got:\n%s", text)
	}
	if strings.Contains(text, "2\tb") {
		t.Errorf("line 2 should not be present, got:\n%s", text)
	}
}

func TestReadTool_WithLimit(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "lines.txt")
	if err := os.WriteFile(filePath, []byte("a\nb\nc\nd\ne\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := ReadTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path":  "lines.txt",
		"limit": float64(2),
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d:\n%s", len(lines), text)
	}
}

func TestReadTool_OffsetBeyondFileLength(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "short.txt")
	if err := os.WriteFile(filePath, []byte("only\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := ReadTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path":   "short.txt",
		"offset": float64(100),
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for offset beyond file length")
	}
	if !strings.Contains(result.Content[0].Text, "Offset") {
		t.Errorf("expected offset error message, got: %s", result.Content[0].Text)
	}
}

func TestReadTool_RequiresPath(t *testing.T) {
	dir := t.TempDir()
	tool := ReadTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when path is missing")
	}
	if !strings.Contains(result.Content[0].Text, "path parameter is required") {
		t.Errorf("unexpected error message: %s", result.Content[0].Text)
	}
}
