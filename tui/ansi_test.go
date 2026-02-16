package tui

import (
	"strings"
	"testing"
)

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "removes bold codes",
			input: BoldOn + "bold" + BoldOff,
			want:  "bold",
		},
		{
			name:  "removes color codes",
			input: FgRed + "red" + Reset,
			want:  "red",
		},
		{
			name:  "removes multiple escape sequences",
			input: FgBlue + BoldOn + "styled" + Reset + " plain " + FgGreen + "green" + Reset,
			want:  "styled plain green",
		},
		{
			name:  "removes cursor movement codes",
			input: CursorUp + "text" + CursorDown,
			want:  "text",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only escape sequences",
			input: BoldOn + Reset + FgRed + Reset,
			want:  "",
		},
		{
			name:  "removes RGB color codes",
			input: "\033[38;2;255;0;0mred\033[0m",
			want:  "red",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripAnsi(tt.input)
			if got != tt.want {
				t.Errorf("StripAnsi(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestVisibleWidth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "plain ASCII",
			input: "hello",
			want:  5,
		},
		{
			name:  "empty string",
			input: "",
			want:  0,
		},
		{
			name:  "with ANSI codes",
			input: FgRed + "hello" + Reset,
			want:  5,
		},
		{
			name:  "CJK characters width 2 each",
			input: "\u4f60\u597d",
			want:  4,
		},
		{
			name:  "mixed ASCII and CJK",
			input: "hi\u4f60\u597d",
			want:  6,
		},
		{
			name:  "CJK with ANSI codes",
			input: FgRed + "\u4f60\u597d" + Reset,
			want:  4,
		},
		{
			name:  "fullwidth forms",
			input: "\uff01\uff02",
			want:  4,
		},
		{
			name:  "hangul syllables",
			input: "\uac00\uac01",
			want:  4,
		},
		{
			name:  "bold wrapped text",
			input: Bold("test"),
			want:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VisibleWidth(tt.input)
			if got != tt.want {
				t.Errorf("VisibleWidth(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		check func(t *testing.T, result string)
	}{
		{
			name:  "plain text within width",
			input: "hello",
			width: 10,
			check: func(t *testing.T, result string) {
				if result != "hello" {
					t.Errorf("expected %q, got %q", "hello", result)
				}
			},
		},
		{
			name:  "plain text truncated",
			input: "hello world this is long",
			width: 10,
			check: func(t *testing.T, result string) {
				stripped := StripAnsi(result)
				vw := VisibleWidth(result)
				if vw > 10 {
					t.Errorf("visible width %d exceeds max width 10", vw)
				}
				if !strings.Contains(stripped, "\u2026") {
					t.Errorf("expected ellipsis in truncated text, got %q", stripped)
				}
			},
		},
		{
			name:  "text with ANSI codes truncated",
			input: FgRed + "hello world this is long" + Reset,
			width: 10,
			check: func(t *testing.T, result string) {
				vw := VisibleWidth(result)
				if vw > 10 {
					t.Errorf("visible width %d exceeds max width 10", vw)
				}
				// Should contain reset at the end since ANSI was opened.
				if !strings.HasSuffix(result, Reset) {
					t.Errorf("expected Reset suffix for ANSI truncation")
				}
			},
		},
		{
			name:  "already short text unchanged",
			input: "hi",
			width: 10,
			check: func(t *testing.T, result string) {
				if result != "hi" {
					t.Errorf("expected %q, got %q", "hi", result)
				}
			},
		},
		{
			name:  "width zero returns empty",
			input: "hello",
			width: 0,
			check: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
			},
		},
		{
			name:  "negative width returns empty",
			input: "hello",
			width: -1,
			check: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateToWidth(tt.input, tt.width)
			tt.check(t, result)
		})
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		check func(t *testing.T, lines []string)
	}{
		{
			name:  "single line within width",
			input: "hello world",
			width: 20,
			check: func(t *testing.T, lines []string) {
				if len(lines) != 1 {
					t.Fatalf("expected 1 line, got %d", len(lines))
				}
				if lines[0] != "hello world" {
					t.Errorf("expected %q, got %q", "hello world", lines[0])
				}
			},
		},
		{
			name:  "wrapping at word boundary",
			input: "hello world foo",
			width: 10,
			check: func(t *testing.T, lines []string) {
				if len(lines) < 2 {
					t.Fatalf("expected at least 2 lines, got %d: %v", len(lines), lines)
				}
				for _, line := range lines {
					if VisibleWidth(line) > 10 {
						t.Errorf("line %q exceeds width 10 (visible width: %d)", line, VisibleWidth(line))
					}
				}
			},
		},
		{
			name:  "long word broken",
			input: "abcdefghijklmnop",
			width: 5,
			check: func(t *testing.T, lines []string) {
				if len(lines) < 2 {
					t.Fatalf("expected long word to be broken into multiple lines, got %d lines", len(lines))
				}
				for _, line := range lines {
					if VisibleWidth(line) > 5 {
						t.Errorf("line %q exceeds width 5", line)
					}
				}
			},
		},
		{
			name:  "ANSI codes preserved across wraps",
			input: FgRed + "hello world foo bar" + Reset,
			width: 10,
			check: func(t *testing.T, lines []string) {
				if len(lines) < 2 {
					t.Fatalf("expected at least 2 lines, got %d", len(lines))
				}
				for _, line := range lines {
					if VisibleWidth(line) > 10 {
						t.Errorf("line %q exceeds width 10", line)
					}
				}
				// First line should contain the ANSI code.
				if !strings.Contains(lines[0], "\033[") {
					t.Errorf("first line should contain ANSI codes")
				}
			},
		},
		{
			name:  "preserves explicit newlines",
			input: "line one\nline two\nline three",
			width: 80,
			check: func(t *testing.T, lines []string) {
				if len(lines) != 3 {
					t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
				}
			},
		},
		{
			name:  "zero width returns nil",
			input: "hello",
			width: 0,
			check: func(t *testing.T, lines []string) {
				if lines != nil {
					t.Errorf("expected nil, got %v", lines)
				}
			},
		},
		{
			name:  "empty string",
			input: "",
			width: 10,
			check: func(t *testing.T, lines []string) {
				if len(lines) != 1 || lines[0] != "" {
					t.Errorf("expected single empty line, got %v", lines)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := WrapText(tt.input, tt.width)
			tt.check(t, lines)
		})
	}
}

func TestBold(t *testing.T) {
	result := Bold("test")
	if !strings.HasPrefix(result, BoldOn) {
		t.Errorf("Bold should start with BoldOn")
	}
	if !strings.HasSuffix(result, BoldOff) {
		t.Errorf("Bold should end with BoldOff")
	}
	if StripAnsi(result) != "test" {
		t.Errorf("Bold text content should be 'test', got %q", StripAnsi(result))
	}
}

func TestDim(t *testing.T) {
	result := Dim("test")
	if !strings.HasPrefix(result, DimOn) {
		t.Errorf("Dim should start with DimOn")
	}
	if !strings.HasSuffix(result, DimOff) {
		t.Errorf("Dim should end with DimOff")
	}
	if StripAnsi(result) != "test" {
		t.Errorf("Dim text content should be 'test', got %q", StripAnsi(result))
	}
}

func TestItalic(t *testing.T) {
	result := Italic("test")
	if !strings.HasPrefix(result, ItalicOn) {
		t.Errorf("Italic should start with ItalicOn")
	}
	if !strings.HasSuffix(result, ItalicOff) {
		t.Errorf("Italic should end with ItalicOff")
	}
	if StripAnsi(result) != "test" {
		t.Errorf("Italic text content should be 'test', got %q", StripAnsi(result))
	}
}

func TestUnderline(t *testing.T) {
	result := Underline("test")
	if !strings.HasPrefix(result, UnderlineOn) {
		t.Errorf("Underline should start with UnderlineOn")
	}
	if !strings.HasSuffix(result, UnderlineOff) {
		t.Errorf("Underline should end with UnderlineOff")
	}
	if StripAnsi(result) != "test" {
		t.Errorf("Underline text content should be 'test', got %q", StripAnsi(result))
	}
}

func TestStrikethrough(t *testing.T) {
	result := Strikethrough("test")
	if !strings.HasPrefix(result, StrikethroughOn) {
		t.Errorf("Strikethrough should start with StrikethroughOn")
	}
	if !strings.HasSuffix(result, StrikethroughOff) {
		t.Errorf("Strikethrough should end with StrikethroughOff")
	}
	if StripAnsi(result) != "test" {
		t.Errorf("Strikethrough text content should be 'test', got %q", StripAnsi(result))
	}
}

func TestInverse(t *testing.T) {
	result := Inverse("test")
	if !strings.HasPrefix(result, InverseOn) {
		t.Errorf("Inverse should start with InverseOn")
	}
	if !strings.HasSuffix(result, InverseOff) {
		t.Errorf("Inverse should end with InverseOff")
	}
	if StripAnsi(result) != "test" {
		t.Errorf("Inverse text content should be 'test', got %q", StripAnsi(result))
	}
}

func TestFgRGB(t *testing.T) {
	result := FgRGB(255, 128, 0, "orange")
	if !strings.Contains(result, "\033[38;2;255;128;0m") {
		t.Errorf("FgRGB should contain RGB escape sequence, got %q", result)
	}
	if !strings.HasSuffix(result, Reset) {
		t.Errorf("FgRGB should end with Reset")
	}
	if StripAnsi(result) != "orange" {
		t.Errorf("FgRGB text content should be 'orange', got %q", StripAnsi(result))
	}
}

func TestBgRGB(t *testing.T) {
	result := BgRGB(0, 128, 255, "blue")
	if !strings.Contains(result, "\033[48;2;0;128;255m") {
		t.Errorf("BgRGB should contain RGB background escape sequence, got %q", result)
	}
	if !strings.HasSuffix(result, Reset) {
		t.Errorf("BgRGB should end with Reset")
	}
	if StripAnsi(result) != "blue" {
		t.Errorf("BgRGB text content should be 'blue', got %q", StripAnsi(result))
	}
}

func TestFgColor(t *testing.T) {
	result := FgColor(196, "red")
	if !strings.Contains(result, "\033[38;5;196m") {
		t.Errorf("FgColor should contain 256-color escape, got %q", result)
	}
	if StripAnsi(result) != "red" {
		t.Errorf("FgColor text content should be 'red', got %q", StripAnsi(result))
	}
}

func TestBgColor(t *testing.T) {
	result := BgColor(21, "blue")
	if !strings.Contains(result, "\033[48;5;21m") {
		t.Errorf("BgColor should contain 256-color bg escape, got %q", result)
	}
	if StripAnsi(result) != "blue" {
		t.Errorf("BgColor text content should be 'blue', got %q", StripAnsi(result))
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  int
	}{
		{
			name:  "pads short string",
			input: "hi",
			width: 10,
			want:  10,
		},
		{
			name:  "string already at width",
			input: "hello",
			width: 5,
			want:  5,
		},
		{
			name:  "string exceeds width unchanged",
			input: "hello world",
			width: 5,
			want:  11,
		},
		{
			name:  "with ANSI codes",
			input: FgRed + "hi" + Reset,
			width: 10,
			want:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PadRight(tt.input, tt.width)
			got := VisibleWidth(result)
			if got != tt.want {
				t.Errorf("PadRight visible width = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPadCenter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  int
	}{
		{
			name:  "centers short string",
			input: "hi",
			width: 10,
			want:  10,
		},
		{
			name:  "string at width unchanged",
			input: "hello",
			width: 5,
			want:  5,
		},
		{
			name:  "string exceeds width unchanged",
			input: "hello world",
			width: 5,
			want:  11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PadCenter(tt.input, tt.width)
			got := VisibleWidth(result)
			if got != tt.want {
				t.Errorf("PadCenter visible width = %d, want %d", got, tt.want)
			}
		})
	}

	// Test that centering adds roughly equal padding on both sides.
	t.Run("centering is balanced", func(t *testing.T) {
		result := PadCenter("ab", 10)
		// "ab" has width 2, so 8 spaces to distribute: 4 left, 4 right.
		if !strings.HasPrefix(result, "    ") {
			t.Errorf("expected 4 spaces of left padding, got %q", result)
		}
		if !strings.HasSuffix(result, "    ") {
			t.Errorf("expected 4 spaces of right padding, got %q", result)
		}
	})
}

func TestCursorTo(t *testing.T) {
	tests := []struct {
		name string
		row  int
		col  int
		want string
	}{
		{
			name: "top left",
			row:  1,
			col:  1,
			want: "\033[1;1H",
		},
		{
			name: "arbitrary position",
			row:  10,
			col:  20,
			want: "\033[10;20H",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CursorTo(tt.row, tt.col)
			if got != tt.want {
				t.Errorf("CursorTo(%d, %d) = %q, want %q", tt.row, tt.col, got, tt.want)
			}
		})
	}
}

func TestCursorMoveHelpers(t *testing.T) {
	if got := CursorMoveUp(3); got != "\033[3A" {
		t.Errorf("CursorMoveUp(3) = %q, want %q", got, "\033[3A")
	}
	if got := CursorMoveDown(2); got != "\033[2B" {
		t.Errorf("CursorMoveDown(2) = %q, want %q", got, "\033[2B")
	}
	if got := CursorMoveRight(5); got != "\033[5C" {
		t.Errorf("CursorMoveRight(5) = %q, want %q", got, "\033[5C")
	}
	if got := CursorMoveLeft(1); got != "\033[1D" {
		t.Errorf("CursorMoveLeft(1) = %q, want %q", got, "\033[1D")
	}
}

func TestRuneWidth(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want int
	}{
		{"ASCII letter", 'A', 1},
		{"ASCII digit", '0', 1},
		{"space", ' ', 1},
		{"CJK ideograph", '\u4e2d', 2},
		{"Hangul", '\uac00', 2},
		{"Fullwidth exclamation", '\uff01', 2},
		{"null byte", 0, 0},
		{"control char", '\n', 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runeWidth(tt.r)
			if got != tt.want {
				t.Errorf("runeWidth(%q) = %d, want %d", tt.r, got, tt.want)
			}
		})
	}
}
