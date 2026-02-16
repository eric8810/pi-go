package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditTool_ReplacesText(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello world\nfoo bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := EditTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path":     "test.txt",
		"old_text": "hello world",
		"new_text": "goodbye world",
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
	if !strings.Contains(string(data), "goodbye world") {
		t.Errorf("expected 'goodbye world' in file, got: %s", string(data))
	}
	if strings.Contains(string(data), "hello world") {
		t.Errorf("'hello world' should have been replaced, file: %s", string(data))
	}

	if !strings.Contains(result.Content[0].Text, "1 replacement") {
		t.Errorf("unexpected result message: %s", result.Content[0].Text)
	}
}

func TestEditTool_OldTextNotFound(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := EditTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path":     "test.txt",
		"old_text": "nonexistent text",
		"new_text": "replacement",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for non-existent old_text")
	}
	if !strings.Contains(result.Content[0].Text, "old_text not found") {
		t.Errorf("unexpected error message: %s", result.Content[0].Text)
	}
}

func TestEditTool_NonUniqueOldText(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("foo bar\nfoo baz\nfoo qux\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := EditTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path":     "test.txt",
		"old_text": "foo",
		"new_text": "replaced",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for non-unique old_text")
	}
	if !strings.Contains(result.Content[0].Text, "found 3 times") {
		t.Errorf("unexpected error message: %s", result.Content[0].Text)
	}
}

func TestEditTool_ReplaceAll(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("foo bar\nfoo baz\nfoo qux\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := EditTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path":        "test.txt",
		"old_text":    "foo",
		"new_text":    "replaced",
		"replace_all": true,
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
	content := string(data)
	if strings.Contains(content, "foo") {
		t.Errorf("all 'foo' should have been replaced, got: %s", content)
	}
	if strings.Count(content, "replaced") != 3 {
		t.Errorf("expected 3 replacements, got content: %s", content)
	}

	if !strings.Contains(result.Content[0].Text, "3 replacement") {
		t.Errorf("unexpected result message: %s", result.Content[0].Text)
	}
}

func TestEditTool_RequiresPathAndOldText(t *testing.T) {
	dir := t.TempDir()
	tool := EditTool(dir)

	// Missing path
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"old_text": "foo",
		"new_text": "bar",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error when path is missing")
	}
	if !strings.Contains(result.Content[0].Text, "path parameter is required") {
		t.Errorf("unexpected error message: %s", result.Content[0].Text)
	}

	// Missing old_text
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err = tool.Execute(context.Background(), "call-2", map[string]any{
		"path":     "test.txt",
		"new_text": "bar",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error when old_text is missing")
	}
	if !strings.Contains(result.Content[0].Text, "old_text parameter is required") {
		t.Errorf("unexpected error message: %s", result.Content[0].Text)
	}
}
