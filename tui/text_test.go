package tui

import (
	"strings"
	"testing"
)

func TestText_RenderSingleLine(t *testing.T) {
	txt := NewText("hello world")
	lines := txt.Render(80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "hello world" {
		t.Errorf("expected 'hello world', got %q", lines[0])
	}
}

func TestText_WrapsLongLines(t *testing.T) {
	txt := NewText("this is a longer line that should be wrapped at the boundary")
	lines := txt.Render(20)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines for wrapped text, got %d", len(lines))
	}
	for i, line := range lines {
		vw := VisibleWidth(line)
		if vw > 20 {
			t.Errorf("line %d exceeds width 20: %q (visible width: %d)", i, line, vw)
		}
	}
}

func TestText_EmptyContent(t *testing.T) {
	txt := NewText("")
	lines := txt.Render(80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line for empty text, got %d", len(lines))
	}
	if lines[0] != "" {
		t.Errorf("expected empty string, got %q", lines[0])
	}
}

func TestText_BoldStyle(t *testing.T) {
	txt := NewText("bold text")
	txt.SetStyle(TextStyle{Bold: true})
	lines := txt.Render(80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], BoldOn) {
		t.Errorf("expected bold style, line: %q", lines[0])
	}
	if StripAnsi(lines[0]) != "bold text" {
		t.Errorf("stripped text should be 'bold text', got %q", StripAnsi(lines[0]))
	}
}

func TestText_DimStyle(t *testing.T) {
	txt := NewText("dim text")
	txt.SetStyle(TextStyle{Dim: true})
	lines := txt.Render(80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], DimOn) {
		t.Errorf("expected dim style, line: %q", lines[0])
	}
}

func TestText_ItalicStyle(t *testing.T) {
	txt := NewText("italic text")
	txt.SetStyle(TextStyle{Italic: true})
	lines := txt.Render(80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], ItalicOn) {
		t.Errorf("expected italic style, line: %q", lines[0])
	}
}

func TestText_ColorStyle(t *testing.T) {
	txt := NewText("colored")
	txt.SetStyle(TextStyle{Color: FgRed})
	lines := txt.Render(80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], FgRed) {
		t.Errorf("expected color in output, line: %q", lines[0])
	}
}

func TestText_AlignLeft(t *testing.T) {
	txt := NewText("left")
	txt.SetAlignment(AlignLeft)
	lines := txt.Render(20)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// Left alignment should not add leading spaces.
	if strings.HasPrefix(lines[0], " ") {
		t.Errorf("left-aligned text should not have leading spaces: %q", lines[0])
	}
}

func TestText_AlignCenter(t *testing.T) {
	txt := NewText("center")
	txt.SetAlignment(AlignCenter)
	lines := txt.Render(20)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "center" has width 6, 20-6=14, so 7 spaces on left.
	if !strings.HasPrefix(lines[0], "       ") {
		t.Errorf("center-aligned text should have leading spaces: %q", lines[0])
	}
	vw := VisibleWidth(lines[0])
	if vw != 20 {
		t.Errorf("center-aligned text visible width should be 20, got %d", vw)
	}
}

func TestText_AlignRight(t *testing.T) {
	txt := NewText("right")
	txt.SetAlignment(AlignRight)
	lines := txt.Render(20)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "right" has width 5, so 15 spaces on left.
	if !strings.HasPrefix(lines[0], strings.Repeat(" ", 15)) {
		t.Errorf("right-aligned text should have 15 leading spaces: %q", lines[0])
	}
}

func TestText_MaxLines(t *testing.T) {
	txt := NewText("line one\nline two\nline three\nline four\nline five")
	txt.SetMaxLines(3)
	lines := txt.Render(80)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines with maxLines=3, got %d: %v", len(lines), lines)
	}
}

func TestText_MaxLinesZeroUnlimited(t *testing.T) {
	txt := NewText("one\ntwo\nthree\nfour\nfive")
	txt.SetMaxLines(0)
	lines := txt.Render(80)
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines with maxLines=0, got %d", len(lines))
	}
}

func TestText_SetContent(t *testing.T) {
	txt := NewText("original")
	lines1 := txt.Render(80)
	if lines1[0] != "original" {
		t.Fatalf("expected 'original', got %q", lines1[0])
	}

	txt.SetContent("updated")
	lines2 := txt.Render(80)
	if lines2[0] != "updated" {
		t.Errorf("expected 'updated', got %q", lines2[0])
	}
}

func TestText_Content(t *testing.T) {
	txt := NewText("test content")
	if txt.Content() != "test content" {
		t.Errorf("Content() should return 'test content', got %q", txt.Content())
	}
}

func TestText_Caching(t *testing.T) {
	txt := NewText("cached content")
	lines1 := txt.Render(80)
	lines2 := txt.Render(80)
	if len(lines1) != len(lines2) {
		t.Fatalf("cached render lengths differ")
	}
	for i := range lines1 {
		if lines1[i] != lines2[i] {
			t.Errorf("line %d differs between renders: %q vs %q", i, lines1[i], lines2[i])
		}
	}
}

// TruncatedText tests

func TestTruncatedText_WithinWidth(t *testing.T) {
	tt := NewTruncatedText("short")
	lines := tt.Render(80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "short" {
		t.Errorf("expected 'short', got %q", lines[0])
	}
}

func TestTruncatedText_Truncates(t *testing.T) {
	tt := NewTruncatedText("this is a very long line that should be truncated")
	lines := tt.Render(15)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	vw := VisibleWidth(lines[0])
	if vw > 15 {
		t.Errorf("visible width %d exceeds max 15", vw)
	}
}

func TestTruncatedText_AppliesStyle(t *testing.T) {
	tt := NewTruncatedText("styled")
	tt.SetStyle(TextStyle{Bold: true})
	lines := tt.Render(80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], BoldOn) {
		t.Errorf("expected bold style in output: %q", lines[0])
	}
	if StripAnsi(lines[0]) != "styled" {
		t.Errorf("stripped text should be 'styled', got %q", StripAnsi(lines[0]))
	}
}

func TestTruncatedText_SetContent(t *testing.T) {
	tt := NewTruncatedText("first")
	if tt.Content() != "first" {
		t.Errorf("expected 'first', got %q", tt.Content())
	}

	tt.SetContent("second")
	if tt.Content() != "second" {
		t.Errorf("expected 'second', got %q", tt.Content())
	}

	lines := tt.Render(80)
	if StripAnsi(lines[0]) != "second" {
		t.Errorf("expected 'second' after SetContent, got %q", StripAnsi(lines[0]))
	}
}

// Spacer tests

func TestSpacer_RendersEmptyLines(t *testing.T) {
	tests := []struct {
		name  string
		lines int
		want  int
	}{
		{"1 line", 1, 1},
		{"3 lines", 3, 3},
		{"5 lines", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSpacer(tt.lines)
			result := s.Render(80)
			if len(result) != tt.want {
				t.Errorf("expected %d empty lines, got %d", tt.want, len(result))
			}
			for i, line := range result {
				if line != "" {
					t.Errorf("line %d should be empty, got %q", i, line)
				}
			}
		})
	}
}

func TestSpacer_MinimumOneLine(t *testing.T) {
	s := NewSpacer(0)
	result := s.Render(80)
	if len(result) != 1 {
		t.Errorf("Spacer(0) should produce 1 line, got %d", len(result))
	}

	s2 := NewSpacer(-5)
	result2 := s2.Render(80)
	if len(result2) != 1 {
		t.Errorf("Spacer(-5) should produce 1 line, got %d", len(result2))
	}
}

func TestSpacer_SetLines(t *testing.T) {
	s := NewSpacer(1)
	result := s.Render(80)
	if len(result) != 1 {
		t.Fatalf("expected 1 line, got %d", len(result))
	}

	s.SetLines(4)
	result = s.Render(80)
	if len(result) != 4 {
		t.Errorf("expected 4 lines after SetLines(4), got %d", len(result))
	}
}

func TestText_UnderlineStyle(t *testing.T) {
	txt := NewText("underlined")
	txt.SetStyle(TextStyle{Underline: true})
	lines := txt.Render(80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], UnderlineOn) {
		t.Errorf("expected underline style, line: %q", lines[0])
	}
}

func TestText_CombinedStyles(t *testing.T) {
	txt := NewText("combo")
	txt.SetStyle(TextStyle{Bold: true, Italic: true, Color: FgGreen})
	lines := txt.Render(80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], BoldOn) {
		t.Errorf("expected bold in combined style")
	}
	if !strings.Contains(lines[0], ItalicOn) {
		t.Errorf("expected italic in combined style")
	}
	if !strings.Contains(lines[0], FgGreen) {
		t.Errorf("expected green color in combined style")
	}
}
