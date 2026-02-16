package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLsTool_ListsDirectoryContents(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bbb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	tool := LsTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "a.txt") {
		t.Errorf("expected a.txt in listing, got:\n%s", text)
	}
	if !strings.Contains(text, "b.txt") {
		t.Errorf("expected b.txt in listing, got:\n%s", text)
	}
	if !strings.Contains(text, "subdir/") {
		t.Errorf("expected subdir/ in listing, got:\n%s", text)
	}
}

func TestLsTool_NonExistentDirectory(t *testing.T) {
	dir := t.TempDir()

	tool := LsTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path": "nonexistent",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for non-existent directory")
	}
	if !strings.Contains(result.Content[0].Text, "Cannot access path") {
		t.Errorf("unexpected error message: %s", result.Content[0].Text)
	}
}

func TestLsTool_NonDirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := LsTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"path": "file.txt",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for non-directory path")
	}
	if !strings.Contains(result.Content[0].Text, "is not a directory") {
		t.Errorf("unexpected error message: %s", result.Content[0].Text)
	}
}

func TestLsTool_Recursive(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "top.txt"), []byte("top\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "nested.txt"), []byte("nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := LsTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"recursive": true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "top.txt") {
		t.Errorf("expected top.txt in recursive listing, got:\n%s", text)
	}
	if !strings.Contains(text, "nested.txt") {
		t.Errorf("expected nested.txt in recursive listing, got:\n%s", text)
	}
}
