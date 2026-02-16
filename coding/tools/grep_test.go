package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepTool_FindsPattern(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("hello world\nfoo bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("goodbye world\nhello again\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := GrepTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"pattern": "hello",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "hello") {
		t.Errorf("expected matches containing 'hello', got:\n%s", text)
	}
	// Should find matches in both files
	if !strings.Contains(text, "file1.txt") {
		t.Errorf("expected match in file1.txt, got:\n%s", text)
	}
	if !strings.Contains(text, "file2.txt") {
		t.Errorf("expected match in file2.txt, got:\n%s", text)
	}
}

func TestGrepTool_IncludeFilter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.go"), []byte("func main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("func not go\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := GrepTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"pattern": "func",
		"include": "*.go",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "file.go") {
		t.Errorf("expected match in file.go, got:\n%s", text)
	}
	if strings.Contains(text, "file.txt") {
		t.Errorf("file.txt should not match with include=*.go, got:\n%s", text)
	}
}

func TestGrepTool_NoMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := GrepTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"pattern": "zzzznotfound",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected non-error result for no matches")
	}
	if !strings.Contains(result.Content[0].Text, "No matches found") {
		t.Errorf("expected 'No matches found' message, got: %s", result.Content[0].Text)
	}
}

func TestGrepTool_InvalidRegex(t *testing.T) {
	dir := t.TempDir()

	tool := GrepTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"pattern": "[invalid",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for invalid regex")
	}
	if !strings.Contains(result.Content[0].Text, "Invalid regex") {
		t.Errorf("unexpected error message: %s", result.Content[0].Text)
	}
}

func TestGrepTool_ContextLines(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nMATCH\nline4\nline5\n"
	if err := os.WriteFile(filepath.Join(dir, "ctx.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := GrepTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"pattern":       "MATCH",
		"context_lines": float64(1),
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	// Should include surrounding context lines
	if !strings.Contains(text, "line2") {
		t.Errorf("expected context line 'line2', got:\n%s", text)
	}
	if !strings.Contains(text, "MATCH") {
		t.Errorf("expected match line 'MATCH', got:\n%s", text)
	}
	if !strings.Contains(text, "line4") {
		t.Errorf("expected context line 'line4', got:\n%s", text)
	}
}
