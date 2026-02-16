package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTool_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()

	tool := WriteTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path":    "newfile.txt",
		"content": "hello world",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	data, err := os.ReadFile(filepath.Join(dir, "newfile.txt"))
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("file content = %q, want %q", string(data), "hello world")
	}

	if !strings.Contains(result.Content[0].Text, "Successfully wrote") {
		t.Errorf("unexpected result message: %s", result.Content[0].Text)
	}
}

func TestWriteTool_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()

	tool := WriteTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path":    "sub/dir/file.txt",
		"content": "nested content",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	data, err := os.ReadFile(filepath.Join(dir, "sub", "dir", "file.txt"))
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if string(data) != "nested content" {
		t.Errorf("file content = %q, want %q", string(data), "nested content")
	}
}

func TestWriteTool_OverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(filePath, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := WriteTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path":    "existing.txt",
		"content": "new content",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("file content = %q, want %q", string(data), "new content")
	}
}

func TestWriteTool_RequiresPath(t *testing.T) {
	dir := t.TempDir()

	tool := WriteTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"content": "some content",
	}, nil)
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
