package tools

import (
	"testing"
)

func TestCodingTools_ReturnsFourTools(t *testing.T) {
	tools := CodingTools("/tmp")
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}

	expected := map[string]bool{"read": true, "bash": true, "edit": true, "write": true}
	for _, tool := range tools {
		if !expected[tool.Name] {
			t.Errorf("unexpected tool name: %s", tool.Name)
		}
		delete(expected, tool.Name)
	}
	if len(expected) > 0 {
		t.Errorf("missing tools: %v", expected)
	}
}

func TestReadOnlyTools_ReturnsFourTools(t *testing.T) {
	tools := ReadOnlyTools("/tmp")
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}

	expected := map[string]bool{"read": true, "grep": true, "find": true, "ls": true}
	for _, tool := range tools {
		if !expected[tool.Name] {
			t.Errorf("unexpected tool name: %s", tool.Name)
		}
		delete(expected, tool.Name)
	}
	if len(expected) > 0 {
		t.Errorf("missing tools: %v", expected)
	}
}

func TestAllTools_ReturnsSevenTools(t *testing.T) {
	tools := AllTools("/tmp")
	if len(tools) != 7 {
		t.Fatalf("expected 7 tools, got %d", len(tools))
	}

	expected := map[string]bool{
		"read": true, "bash": true, "edit": true, "write": true,
		"grep": true, "find": true, "ls": true,
	}
	for _, tool := range tools {
		if !expected[tool.Name] {
			t.Errorf("unexpected tool name: %s", tool.Name)
		}
		delete(expected, tool.Name)
	}
	if len(expected) > 0 {
		t.Errorf("missing tools: %v", expected)
	}
}

func TestToolSets_CorrectNames(t *testing.T) {
	// Verify that CodingTools contains exactly read, bash, edit, write
	coding := CodingTools("/tmp")
	codingNames := make([]string, len(coding))
	for i, tool := range coding {
		codingNames[i] = tool.Name
	}

	expectedCoding := []string{"read", "bash", "edit", "write"}
	if len(codingNames) != len(expectedCoding) {
		t.Fatalf("CodingTools: expected %d tools, got %d", len(expectedCoding), len(codingNames))
	}
	for i, name := range expectedCoding {
		if codingNames[i] != name {
			t.Errorf("CodingTools[%d]: expected %q, got %q", i, name, codingNames[i])
		}
	}

	// Verify that ReadOnlyTools contains exactly read, grep, find, ls
	readonly := ReadOnlyTools("/tmp")
	readonlyNames := make([]string, len(readonly))
	for i, tool := range readonly {
		readonlyNames[i] = tool.Name
	}

	expectedReadonly := []string{"read", "grep", "find", "ls"}
	if len(readonlyNames) != len(expectedReadonly) {
		t.Fatalf("ReadOnlyTools: expected %d tools, got %d", len(expectedReadonly), len(readonlyNames))
	}
	for i, name := range expectedReadonly {
		if readonlyNames[i] != name {
			t.Errorf("ReadOnlyTools[%d]: expected %q, got %q", i, name, readonlyNames[i])
		}
	}
}
