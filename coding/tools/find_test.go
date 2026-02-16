package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindTool_LocatesFilesByPattern(t *testing.T) {
	dir := t.TempDir()
	// Create some files
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "util.go"), []byte("package util\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := FindTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"pattern": "*.go",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "main.go") {
		t.Errorf("expected to find main.go, got:\n%s", text)
	}
	if !strings.Contains(text, "util.go") {
		t.Errorf("expected to find util.go, got:\n%s", text)
	}
	if strings.Contains(text, "readme.md") {
		t.Errorf("readme.md should not match *.go, got:\n%s", text)
	}
}

func TestFindTool_NoMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := FindTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"pattern": "*.rs",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected non-error result for no matches")
	}
	if !strings.Contains(result.Content[0].Text, "No files found") {
		t.Errorf("expected 'No files found' message, got: %s", result.Content[0].Text)
	}
}

func TestFindTool_SkipsGitDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a .git directory with a file
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config.txt"), []byte("git config\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a normal file
	if err := os.WriteFile(filepath.Join(dir, "config.txt"), []byte("app config\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := FindTool(dir)
	result, err := tool.Execute(context.Background(), "call-1", map[string]any{
		"pattern": "*.txt",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "config.txt") {
		t.Errorf("expected to find config.txt, got:\n%s", text)
	}
	if strings.Contains(text, ".git") {
		t.Errorf("should not include .git files, got:\n%s", text)
	}
}
