package tui

import (
	"strings"
	"testing"
)

func TestEditor_RenderEmpty(t *testing.T) {
	ed := NewEditor()
	lines := ed.Render(80)
	if len(lines) == 0 {
		t.Fatal("editor should produce at least one line")
	}
}

func TestEditor_SetText(t *testing.T) {
	ed := NewEditor()
	ed.SetText("line one\nline two\nline three")

	text := ed.Text()
	if text != "line one\nline two\nline three" {
		t.Errorf("expected multi-line text, got %q", text)
	}
}

func TestEditor_TextJoined(t *testing.T) {
	ed := NewEditor()
	ed.SetText("a\nb\nc")
	got := ed.Text()
	if got != "a\nb\nc" {
		t.Errorf("Text() should return joined lines, got %q", got)
	}
}

func TestEditor_HandleInput_PrintableChars(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)

	ed.HandleInput([]byte{'h'})
	ed.HandleInput([]byte{'i'})

	if ed.Text() != "hi" {
		t.Errorf("expected 'hi', got %q", ed.Text())
	}
}

func TestEditor_HandleInput_Enter_InsertsNewline(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)
	ed.SetUseShiftEnterSubmit(true) // Enter inserts newline in this mode.

	ed.HandleInput([]byte{'a'})
	ed.HandleInput([]byte{'\r'}) // Enter
	ed.HandleInput([]byte{'b'})

	if ed.Text() != "a\nb" {
		t.Errorf("expected 'a\\nb', got %q", ed.Text())
	}
	if ed.LineCount() != 2 {
		t.Errorf("expected 2 lines, got %d", ed.LineCount())
	}
}

func TestEditor_HandleInput_ShiftEnter_InsertsNewline(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)
	// Default mode: Enter submits, Shift+Enter inserts newline.

	ed.HandleInput([]byte{'a'})
	ed.HandleInput([]byte("\x1b\r")) // Shift+Enter
	ed.HandleInput([]byte{'b'})

	if ed.Text() != "a\nb" {
		t.Errorf("expected 'a\\nb', got %q", ed.Text())
	}
}

func TestEditor_HandleInput_ArrowKeys(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)
	ed.SetText("ab\ncd")

	// Cursor should be at end of "cd" (row 1, col 2).
	row, col := ed.CursorPosition()
	if row != 1 || col != 2 {
		t.Errorf("expected cursor at (1,2), got (%d,%d)", row, col)
	}

	// Move left.
	ed.HandleInput([]byte{0x1b, '[', 'D'})
	row, col = ed.CursorPosition()
	if row != 1 || col != 1 {
		t.Errorf("expected cursor at (1,1) after left, got (%d,%d)", row, col)
	}

	// Move up.
	ed.HandleInput([]byte{0x1b, '[', 'A'})
	row, col = ed.CursorPosition()
	if row != 0 || col != 1 {
		t.Errorf("expected cursor at (0,1) after up, got (%d,%d)", row, col)
	}

	// Move right.
	ed.HandleInput([]byte{0x1b, '[', 'C'})
	row, col = ed.CursorPosition()
	if row != 0 || col != 2 {
		t.Errorf("expected cursor at (0,2) after right, got (%d,%d)", row, col)
	}

	// Move down.
	ed.HandleInput([]byte{0x1b, '[', 'B'})
	row, col = ed.CursorPosition()
	if row != 1 {
		t.Errorf("expected cursor at row 1 after down, got row %d", row)
	}
}

func TestEditor_HandleInput_BackspaceJoinsLines(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)
	ed.SetText("ab\ncd")

	// Cursor is at end of "cd" (row 1, col 2).
	// Move to start of second line.
	ed.HandleInput([]byte("\x1b[H")) // Home
	row, col := ed.CursorPosition()
	if row != 1 || col != 0 {
		t.Errorf("expected cursor at (1,0) after Home, got (%d,%d)", row, col)
	}

	// Backspace at start of line 2 should join with line 1.
	ed.HandleInput([]byte{0x7f})
	if ed.Text() != "abcd" {
		t.Errorf("expected 'abcd' after backspace join, got %q", ed.Text())
	}
	if ed.LineCount() != 1 {
		t.Errorf("expected 1 line after join, got %d", ed.LineCount())
	}
	row, col = ed.CursorPosition()
	if row != 0 || col != 2 {
		t.Errorf("expected cursor at (0,2) after join, got (%d,%d)", row, col)
	}
}

func TestEditor_HandleInput_BackspaceDeletesChar(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)
	ed.SetText("abc")

	ed.HandleInput([]byte{0x7f})
	if ed.Text() != "ab" {
		t.Errorf("expected 'ab' after backspace, got %q", ed.Text())
	}
}

func TestEditor_CursorPosition(t *testing.T) {
	ed := NewEditor()
	ed.SetText("first\nsecond\nthird")

	row, col := ed.CursorPosition()
	// SetText moves cursor to end.
	if row != 2 {
		t.Errorf("expected cursor row 2, got %d", row)
	}
	if col != 5 {
		t.Errorf("expected cursor col 5, got %d", col)
	}
}

func TestEditor_LineCount(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"single line", "hello", 1},
		{"two lines", "a\nb", 2},
		{"three lines", "a\nb\nc", 3},
		{"empty editor", "", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed := NewEditor()
			if tt.text != "" {
				ed.SetText(tt.text)
			}
			if ed.LineCount() != tt.want {
				t.Errorf("LineCount() = %d, want %d", ed.LineCount(), tt.want)
			}
		})
	}
}

func TestEditor_Focus(t *testing.T) {
	ed := NewEditor()
	if ed.IsFocused() {
		t.Error("editor should not be focused by default")
	}

	ed.SetFocused(true)
	if !ed.IsFocused() {
		t.Error("editor should be focused after SetFocused(true)")
	}

	ed.SetFocused(false)
	if ed.IsFocused() {
		t.Error("editor should not be focused after SetFocused(false)")
	}
}

func TestEditor_OnChange(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)

	var changes []string
	ed.SetOnChange(func(text string) {
		changes = append(changes, text)
	})

	ed.HandleInput([]byte{'x'})
	ed.HandleInput([]byte{'y'})

	if len(changes) != 2 {
		t.Fatalf("expected 2 onChange calls, got %d", len(changes))
	}
	if changes[0] != "x" {
		t.Errorf("first change should be 'x', got %q", changes[0])
	}
	if changes[1] != "xy" {
		t.Errorf("second change should be 'xy', got %q", changes[1])
	}
}

func TestEditor_OnSubmit(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)
	ed.SetText("submit me")

	var submitted string
	ed.SetOnSubmit(func(text string) {
		submitted = text
	})

	// Default mode: Enter submits.
	ed.HandleInput([]byte{'\r'})
	if submitted != "submit me" {
		t.Errorf("expected 'submit me', got %q", submitted)
	}
}

func TestEditor_MaxVisible(t *testing.T) {
	ed := NewEditor()
	ed.SetMaxVisible(3)
	ed.SetText("line1\nline2\nline3\nline4\nline5")

	lines := ed.Render(80)
	if len(lines) != 3 {
		t.Errorf("expected 3 visible lines with maxVisible=3, got %d", len(lines))
	}
}

func TestEditor_HomeEnd(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)
	ed.SetText("hello")

	// Home moves to start.
	ed.HandleInput([]byte("\x1b[H"))
	row, col := ed.CursorPosition()
	if col != 0 {
		t.Errorf("expected col 0 after Home, got %d", col)
	}
	if row != 0 {
		t.Errorf("expected row 0 after Home, got %d", row)
	}

	// End moves to end.
	ed.HandleInput([]byte("\x1b[F"))
	_, col = ed.CursorPosition()
	if col != 5 {
		t.Errorf("expected col 5 after End, got %d", col)
	}
}

func TestEditor_MoveLeftWraps(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)
	ed.SetText("ab\ncd")

	// Move to start of second line.
	ed.HandleInput([]byte("\x1b[H"))
	row, col := ed.CursorPosition()
	if row != 1 || col != 0 {
		t.Fatalf("expected (1,0), got (%d,%d)", row, col)
	}

	// Left should wrap to end of first line.
	ed.HandleInput([]byte{0x1b, '[', 'D'})
	row, col = ed.CursorPosition()
	if row != 0 || col != 2 {
		t.Errorf("expected (0,2) after left wrap, got (%d,%d)", row, col)
	}
}

func TestEditor_MoveRightWraps(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)
	ed.SetText("ab\ncd")

	// Move to end of first line.
	ed.HandleInput([]byte{0x1b, '[', 'A'}) // Up to first line
	ed.HandleInput([]byte("\x1b[F"))        // End

	row, col := ed.CursorPosition()
	if row != 0 || col != 2 {
		t.Fatalf("expected (0,2), got (%d,%d)", row, col)
	}

	// Right should wrap to start of next line.
	ed.HandleInput([]byte{0x1b, '[', 'C'})
	row, col = ed.CursorPosition()
	if row != 1 || col != 0 {
		t.Errorf("expected (1,0) after right wrap, got (%d,%d)", row, col)
	}
}

func TestEditor_RenderFocused(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)
	ed.SetText("test")

	lines := ed.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// Focused line should show cursor (inverse).
	if !strings.Contains(lines[len(lines)-1], InverseOn) {
		t.Errorf("focused editor should show cursor: %q", lines[len(lines)-1])
	}
}

func TestEditor_DeleteLine(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)
	ed.SetText("first\nsecond\nthird")

	// Move to second line.
	ed.HandleInput([]byte{0x1b, '[', 'A'}) // Up

	// Ctrl+D deletes current line.
	ed.HandleInput([]byte{0x04})
	if ed.LineCount() != 2 {
		t.Errorf("expected 2 lines after delete, got %d", ed.LineCount())
	}
}

func TestEditor_CtrlK(t *testing.T) {
	ed := NewEditor()
	ed.SetFocused(true)
	ed.SetText("hello world")

	// Move to middle.
	ed.HandleInput([]byte("\x1b[H"))
	for i := 0; i < 5; i++ {
		ed.HandleInput([]byte{0x1b, '[', 'C'})
	}

	// Ctrl+K kills to end.
	ed.HandleInput([]byte{0x0b})
	if ed.Text() != "hello" {
		t.Errorf("expected 'hello' after Ctrl+K, got %q", ed.Text())
	}
}

func TestEditor_ZeroWidth(t *testing.T) {
	ed := NewEditor()
	ed.SetText("test")
	lines := ed.Render(0)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
}
