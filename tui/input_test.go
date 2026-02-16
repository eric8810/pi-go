package tui

import (
	"strings"
	"testing"
)

func TestInput_RenderEmptyWithPlaceholder(t *testing.T) {
	inp := NewInput()
	inp.SetPlaceholder("Type here...")

	lines := inp.Render(40)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	stripped := StripAnsi(lines[0])
	if !strings.Contains(stripped, "Type here...") {
		t.Errorf("empty input should show placeholder: %q", stripped)
	}
	// Placeholder should be dim.
	if !strings.Contains(lines[0], DimOn) {
		t.Errorf("placeholder should be dim: %q", lines[0])
	}
}

func TestInput_RenderEmptyFocusedWithPlaceholder(t *testing.T) {
	inp := NewInput()
	inp.SetPlaceholder("Type here...")
	inp.SetFocused(true)

	lines := inp.Render(40)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// Should show cursor (inverse) and placeholder.
	if !strings.Contains(lines[0], InverseOn) {
		t.Errorf("focused empty input should show cursor: %q", lines[0])
	}
}

func TestInput_RenderEmptyFocusedNoPlaceholder(t *testing.T) {
	inp := NewInput()
	inp.SetFocused(true)

	lines := inp.Render(40)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], InverseOn) {
		t.Errorf("focused empty input should show cursor: %q", lines[0])
	}
}

func TestInput_RenderTextContent(t *testing.T) {
	inp := NewInput()
	inp.SetText("hello world")

	lines := inp.Render(40)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	stripped := StripAnsi(lines[0])
	if !strings.Contains(stripped, "hello world") {
		t.Errorf("input should show text content: %q", stripped)
	}
}

func TestInput_HandleInput_PrintableChars(t *testing.T) {
	inp := NewInput()
	inp.SetFocused(true)

	consumed := inp.HandleInput([]byte{'h'})
	if !consumed {
		t.Error("printable char should be consumed")
	}
	consumed = inp.HandleInput([]byte{'i'})
	if !consumed {
		t.Error("printable char should be consumed")
	}

	if inp.Text() != "hi" {
		t.Errorf("expected 'hi', got %q", inp.Text())
	}
}

func TestInput_HandleInput_Backspace(t *testing.T) {
	inp := NewInput()
	inp.SetText("abc")
	inp.SetFocused(true)

	consumed := inp.HandleInput([]byte{0x7f})
	if !consumed {
		t.Error("backspace should be consumed")
	}

	if inp.Text() != "ab" {
		t.Errorf("expected 'ab' after backspace, got %q", inp.Text())
	}
}

func TestInput_HandleInput_BackspaceEmpty(t *testing.T) {
	inp := NewInput()
	inp.SetFocused(true)

	consumed := inp.HandleInput([]byte{0x7f})
	if !consumed {
		t.Error("backspace should be consumed even when empty")
	}
	if inp.Text() != "" {
		t.Errorf("expected empty string, got %q", inp.Text())
	}
}

func TestInput_HandleInput_Delete(t *testing.T) {
	inp := NewInput()
	inp.SetText("abc")
	inp.SetFocused(true)

	// Move cursor to beginning.
	inp.HandleInput([]byte("\x1b[H")) // Home key

	// Delete forward.
	consumed := inp.HandleInput([]byte("\x1b[3~"))
	if !consumed {
		t.Error("delete should be consumed")
	}
	if inp.Text() != "bc" {
		t.Errorf("expected 'bc' after delete, got %q", inp.Text())
	}
}

func TestInput_HandleInput_ArrowKeys(t *testing.T) {
	inp := NewInput()
	inp.SetText("abc")
	inp.SetFocused(true)

	// Cursor is at end (position 3).
	consumed := inp.HandleInput([]byte{0x1b, '[', 'D'}) // Left
	if !consumed {
		t.Error("left arrow should be consumed")
	}

	// Now insert a character at position 2.
	inp.HandleInput([]byte{'X'})
	if inp.Text() != "abXc" {
		t.Errorf("expected 'abXc', got %q", inp.Text())
	}
}

func TestInput_HandleInput_LeftAtStart(t *testing.T) {
	inp := NewInput()
	inp.SetText("abc")
	inp.SetFocused(true)

	// Move to start.
	inp.HandleInput([]byte("\x1b[H"))
	// Left at start should be no-op.
	inp.HandleInput([]byte{0x1b, '[', 'D'})
	// Insert at start.
	inp.HandleInput([]byte{'X'})
	if inp.Text() != "Xabc" {
		t.Errorf("expected 'Xabc', got %q", inp.Text())
	}
}

func TestInput_HandleInput_RightAtEnd(t *testing.T) {
	inp := NewInput()
	inp.SetText("abc")
	inp.SetFocused(true)

	// Right at end should be no-op.
	inp.HandleInput([]byte{0x1b, '[', 'C'})
	inp.HandleInput([]byte{'X'})
	if inp.Text() != "abcX" {
		t.Errorf("expected 'abcX', got %q", inp.Text())
	}
}

func TestInput_HandleInput_HomeEnd(t *testing.T) {
	inp := NewInput()
	inp.SetText("hello")
	inp.SetFocused(true)

	// Home moves to start.
	inp.HandleInput([]byte("\x1b[H"))
	inp.HandleInput([]byte{'A'})
	if inp.Text() != "Ahello" {
		t.Errorf("expected 'Ahello' after Home, got %q", inp.Text())
	}

	// End moves to end.
	inp.HandleInput([]byte("\x1b[F"))
	inp.HandleInput([]byte{'Z'})
	if inp.Text() != "AhelloZ" {
		t.Errorf("expected 'AhelloZ' after End, got %q", inp.Text())
	}
}

func TestInput_HandleInput_CtrlA_CtrlE(t *testing.T) {
	inp := NewInput()
	inp.SetText("test")
	inp.SetFocused(true)

	// Ctrl+A moves to start.
	inp.HandleInput([]byte{0x01})
	inp.HandleInput([]byte{'X'})
	if inp.Text() != "Xtest" {
		t.Errorf("expected 'Xtest' after Ctrl+A, got %q", inp.Text())
	}

	// Ctrl+E moves to end.
	inp.HandleInput([]byte{0x05})
	inp.HandleInput([]byte{'Y'})
	if inp.Text() != "XtestY" {
		t.Errorf("expected 'XtestY' after Ctrl+E, got %q", inp.Text())
	}
}

func TestInput_HandleInput_Enter(t *testing.T) {
	inp := NewInput()
	inp.SetText("submitted text")
	inp.SetFocused(true)

	var submitted string
	inp.SetOnSubmit(func(text string) {
		submitted = text
	})

	consumed := inp.HandleInput([]byte{'\r'})
	if !consumed {
		t.Error("enter should be consumed")
	}
	if submitted != "submitted text" {
		t.Errorf("expected 'submitted text', got %q", submitted)
	}
}

func TestInput_HandleInput_EnterNoCallback(t *testing.T) {
	inp := NewInput()
	inp.SetText("text")
	inp.SetFocused(true)

	// Should not panic when there's no callback.
	consumed := inp.HandleInput([]byte{'\r'})
	if !consumed {
		t.Error("enter should be consumed even without callback")
	}
}

func TestInput_SetText_And_Text(t *testing.T) {
	inp := NewInput()
	inp.SetText("test value")
	if inp.Text() != "test value" {
		t.Errorf("expected 'test value', got %q", inp.Text())
	}

	inp.SetText("new value")
	if inp.Text() != "new value" {
		t.Errorf("expected 'new value', got %q", inp.Text())
	}
}

func TestInput_OnChange(t *testing.T) {
	inp := NewInput()
	inp.SetFocused(true)

	var changes []string
	inp.SetOnChange(func(text string) {
		changes = append(changes, text)
	})

	inp.HandleInput([]byte{'a'})
	inp.HandleInput([]byte{'b'})

	if len(changes) != 2 {
		t.Fatalf("expected 2 onChange calls, got %d", len(changes))
	}
	if changes[0] != "a" {
		t.Errorf("first change should be 'a', got %q", changes[0])
	}
	if changes[1] != "ab" {
		t.Errorf("second change should be 'ab', got %q", changes[1])
	}
}

func TestInput_Focus(t *testing.T) {
	inp := NewInput()
	if inp.IsFocused() {
		t.Error("input should not be focused by default")
	}

	inp.SetFocused(true)
	if !inp.IsFocused() {
		t.Error("input should be focused after SetFocused(true)")
	}

	inp.SetFocused(false)
	if inp.IsFocused() {
		t.Error("input should not be focused after SetFocused(false)")
	}
}

func TestInput_HandleInput_CtrlK(t *testing.T) {
	inp := NewInput()
	inp.SetText("hello world")
	inp.SetFocused(true)

	// Move cursor to after "hello" (5 characters).
	inp.HandleInput([]byte("\x1b[H")) // Home
	for i := 0; i < 5; i++ {
		inp.HandleInput([]byte{0x1b, '[', 'C'}) // Right
	}

	// Ctrl+K kills to end.
	inp.HandleInput([]byte{0x0b})
	if inp.Text() != "hello" {
		t.Errorf("expected 'hello' after Ctrl+K, got %q", inp.Text())
	}
}

func TestInput_HandleInput_CtrlU(t *testing.T) {
	inp := NewInput()
	inp.SetText("hello world")
	inp.SetFocused(true)

	// Cursor is at end. Ctrl+U clears before cursor.
	inp.HandleInput([]byte{0x15})
	if inp.Text() != "" {
		t.Errorf("expected empty after Ctrl+U from end, got %q", inp.Text())
	}
}

func TestInput_HandleInput_CtrlW(t *testing.T) {
	inp := NewInput()
	inp.SetText("hello world")
	inp.SetFocused(true)

	// Ctrl+W deletes word before cursor (at end).
	inp.HandleInput([]byte{0x17})
	if inp.Text() != "hello " {
		t.Errorf("expected 'hello ' after Ctrl+W, got %q", inp.Text())
	}
}

func TestInput_HandleInput_UnhandledKey(t *testing.T) {
	inp := NewInput()
	inp.SetFocused(true)

	// Up arrow is not handled by Input.
	consumed := inp.HandleInput([]byte{0x1b, '[', 'A'})
	if consumed {
		t.Error("up arrow should not be consumed by Input")
	}
}

func TestInput_ZeroWidth(t *testing.T) {
	inp := NewInput()
	inp.SetText("test")
	lines := inp.Render(0)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}
