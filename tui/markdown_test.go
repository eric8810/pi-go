package tui

import (
	"strings"
	"testing"
)

func TestMarkdown_EmptyInput(t *testing.T) {
	md := NewMarkdown("")
	lines := md.Render(80)
	// Empty input should produce at least an empty line.
	if len(lines) == 0 {
		t.Fatal("expected at least one line for empty markdown")
	}
}

func TestMarkdown_Headers(t *testing.T) {
	tests := []struct {
		name   string
		source string
		check  func(t *testing.T, lines []string)
	}{
		{
			name:   "H1 header",
			source: "# Title",
			check: func(t *testing.T, lines []string) {
				if len(lines) == 0 {
					t.Fatal("expected at least one line")
				}
				stripped := StripAnsi(lines[0])
				if !strings.Contains(stripped, "Title") {
					t.Errorf("H1 should contain 'Title', got %q", stripped)
				}
				// H1 should have bold and bright white.
				if !strings.Contains(lines[0], BoldOn) {
					t.Errorf("H1 should be bold")
				}
				if !strings.Contains(lines[0], FgBrightWhite) {
					t.Errorf("H1 should be bright white")
				}
			},
		},
		{
			name:   "H2 header",
			source: "## Subtitle",
			check: func(t *testing.T, lines []string) {
				if len(lines) == 0 {
					t.Fatal("expected at least one line")
				}
				stripped := StripAnsi(lines[0])
				if !strings.Contains(stripped, "Subtitle") {
					t.Errorf("H2 should contain 'Subtitle', got %q", stripped)
				}
				if !strings.Contains(lines[0], FgBrightCyan) {
					t.Errorf("H2 should be bright cyan")
				}
			},
		},
		{
			name:   "H3 header",
			source: "### Section",
			check: func(t *testing.T, lines []string) {
				if len(lines) == 0 {
					t.Fatal("expected at least one line")
				}
				stripped := StripAnsi(lines[0])
				if !strings.Contains(stripped, "Section") {
					t.Errorf("H3 should contain 'Section', got %q", stripped)
				}
				if !strings.Contains(lines[0], FgCyan) {
					t.Errorf("H3 should be cyan")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := NewMarkdown(tt.source)
			lines := md.Render(80)
			tt.check(t, lines)
		})
	}
}

func TestMarkdown_BoldText(t *testing.T) {
	md := NewMarkdown("This is **bold** text")
	lines := md.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if !strings.Contains(lines[0], BoldOn) {
		t.Errorf("expected bold ANSI code in output: %q", lines[0])
	}
	stripped := StripAnsi(lines[0])
	if !strings.Contains(stripped, "bold") {
		t.Errorf("expected 'bold' in output: %q", stripped)
	}
	// Double star markers should be removed.
	if strings.Contains(stripped, "**") {
		t.Errorf("markdown markers ** should be removed: %q", stripped)
	}
}

func TestMarkdown_ItalicText(t *testing.T) {
	md := NewMarkdown("This is *italic* text")
	lines := md.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if !strings.Contains(lines[0], ItalicOn) {
		t.Errorf("expected italic ANSI code in output: %q", lines[0])
	}
	stripped := StripAnsi(lines[0])
	if !strings.Contains(stripped, "italic") {
		t.Errorf("expected 'italic' in output: %q", stripped)
	}
}

func TestMarkdown_InlineCode(t *testing.T) {
	md := NewMarkdown("Use `code` here")
	lines := md.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	stripped := StripAnsi(lines[0])
	if !strings.Contains(stripped, "code") {
		t.Errorf("expected 'code' in output: %q", stripped)
	}
	// Backticks should be removed.
	if strings.Contains(stripped, "`") {
		t.Errorf("backticks should be removed: %q", stripped)
	}
	// Should contain background color for code.
	if !strings.Contains(lines[0], "\033[48;5;238m") {
		t.Errorf("inline code should have background color: %q", lines[0])
	}
}

func TestMarkdown_CodeBlock(t *testing.T) {
	source := "```go\nfunc main() {}\n```"
	md := NewMarkdown(source)
	lines := md.Render(80)
	if len(lines) < 3 {
		t.Fatalf("code block should produce at least 3 lines (top, content, bottom), got %d: %v", len(lines), lines)
	}
	// All code block lines should have the dark background.
	bgColor := "\033[48;5;236m"
	for _, line := range lines {
		if !strings.Contains(line, bgColor) {
			t.Errorf("code block line should have background color: %q", line)
		}
	}
	// First line should mention the language.
	stripped := StripAnsi(lines[0])
	if !strings.Contains(stripped, "go") {
		t.Errorf("code block should show language 'go': %q", stripped)
	}
}

func TestMarkdown_CodeBlockNoLanguage(t *testing.T) {
	source := "```\nsome code\n```"
	md := NewMarkdown(source)
	lines := md.Render(80)
	if len(lines) < 3 {
		t.Fatalf("code block should produce at least 3 lines, got %d", len(lines))
	}
}

func TestMarkdown_UnorderedList(t *testing.T) {
	source := "- item one\n- item two\n- item three"
	md := NewMarkdown(source)
	lines := md.Render(80)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines for 3 list items, got %d: %v", len(lines), lines)
	}
	// Each line should contain a bullet character.
	for _, line := range lines {
		stripped := StripAnsi(line)
		if !strings.Contains(stripped, "\u2022") {
			t.Errorf("unordered list item should contain bullet, got %q", stripped)
		}
	}
}

func TestMarkdown_UnorderedListStar(t *testing.T) {
	source := "* first\n* second"
	md := NewMarkdown(source)
	lines := md.Render(80)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	for _, line := range lines {
		stripped := StripAnsi(line)
		if !strings.Contains(stripped, "\u2022") {
			t.Errorf("unordered list item should contain bullet, got %q", stripped)
		}
	}
}

func TestMarkdown_OrderedList(t *testing.T) {
	source := "1. first\n2. second\n3. third"
	md := NewMarkdown(source)
	lines := md.Render(80)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines for ordered list, got %d: %v", len(lines), lines)
	}
	stripped0 := StripAnsi(lines[0])
	if !strings.Contains(stripped0, "1.") {
		t.Errorf("first ordered list item should contain '1.': %q", stripped0)
	}
	stripped1 := StripAnsi(lines[1])
	if !strings.Contains(stripped1, "2.") {
		t.Errorf("second ordered list item should contain '2.': %q", stripped1)
	}
}

func TestMarkdown_Links(t *testing.T) {
	md := NewMarkdown("Visit [Google](https://google.com) now")
	lines := md.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	stripped := StripAnsi(lines[0])
	if !strings.Contains(stripped, "Google") {
		t.Errorf("link text should contain 'Google': %q", stripped)
	}
	if !strings.Contains(stripped, "https://google.com") {
		t.Errorf("link URL should be shown: %q", stripped)
	}
	// Square brackets should be removed.
	if strings.Contains(stripped, "[Google]") {
		t.Errorf("markdown link syntax should be processed: %q", stripped)
	}
}

func TestMarkdown_HorizontalRule(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"dashes", "---"},
		{"stars", "***"},
		{"underscores", "___"},
		{"long dashes", "----------"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := NewMarkdown(tt.source)
			lines := md.Render(40)
			if len(lines) == 0 {
				t.Fatal("expected at least one line for horizontal rule")
			}
			stripped := StripAnsi(lines[0])
			if !strings.Contains(stripped, "\u2500") {
				t.Errorf("horizontal rule should contain dash character, got %q", stripped)
			}
		})
	}
}

func TestMarkdown_BlockQuote(t *testing.T) {
	md := NewMarkdown("> This is a quote")
	lines := md.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	stripped := StripAnsi(lines[0])
	if !strings.Contains(stripped, "\u2502") {
		t.Errorf("block quote should contain vertical border, got %q", stripped)
	}
	if !strings.Contains(stripped, "This is a quote") {
		t.Errorf("block quote should contain content: %q", stripped)
	}
}

func TestMarkdown_EmptyBlockQuote(t *testing.T) {
	md := NewMarkdown(">")
	lines := md.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected at least one line for empty block quote")
	}
}

func TestMarkdown_MixedContent(t *testing.T) {
	source := `# Title

Some **bold** and *italic* text.

- Item one
- Item two

> A quote

---

1. First
2. Second

` + "```\ncode\n```"

	md := NewMarkdown(source)
	lines := md.Render(60)
	if len(lines) < 5 {
		t.Fatalf("mixed content should produce multiple lines, got %d", len(lines))
	}
}

func TestMarkdown_Strikethrough(t *testing.T) {
	md := NewMarkdown("This is ~~deleted~~ text")
	lines := md.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if !strings.Contains(lines[0], StrikethroughOn) {
		t.Errorf("expected strikethrough ANSI code: %q", lines[0])
	}
	stripped := StripAnsi(lines[0])
	if strings.Contains(stripped, "~~") {
		t.Errorf("markdown strikethrough markers should be removed: %q", stripped)
	}
}

func TestMarkdown_SetSource(t *testing.T) {
	md := NewMarkdown("# First")
	lines1 := md.Render(80)

	md.SetSource("# Second")
	lines2 := md.Render(80)

	stripped1 := StripAnsi(lines1[0])
	stripped2 := StripAnsi(lines2[0])

	if !strings.Contains(stripped1, "First") {
		t.Errorf("first render should contain 'First': %q", stripped1)
	}
	if !strings.Contains(stripped2, "Second") {
		t.Errorf("second render should contain 'Second': %q", stripped2)
	}
}

func TestMarkdown_Source(t *testing.T) {
	md := NewMarkdown("# Test")
	if md.Source() != "# Test" {
		t.Errorf("Source() should return '# Test', got %q", md.Source())
	}
}

func TestMarkdown_UnclosedCodeBlock(t *testing.T) {
	source := "```\nsome code without closing"
	md := NewMarkdown(source)
	lines := md.Render(80)
	// Should not panic and should render something.
	if len(lines) == 0 {
		t.Fatal("unclosed code block should still produce output")
	}
}

func TestMarkdown_ParagraphText(t *testing.T) {
	md := NewMarkdown("Just a plain paragraph of text.")
	lines := md.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	stripped := StripAnsi(lines[0])
	if stripped != "Just a plain paragraph of text." {
		t.Errorf("expected plain text, got %q", stripped)
	}
}

func TestMarkdown_EmptyLines(t *testing.T) {
	md := NewMarkdown("first\n\nsecond")
	lines := md.Render(80)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (text, empty, text), got %d: %v", len(lines), lines)
	}
	if lines[1] != "" {
		t.Errorf("middle line should be empty, got %q", lines[1])
	}
}

func TestMarkdown_ZeroWidth(t *testing.T) {
	md := NewMarkdown("# Test")
	lines := md.Render(0)
	if len(lines) == 0 {
		t.Fatal("zero width should still produce output")
	}
}
