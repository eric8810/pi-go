package tui

import (
	"testing"
)

func TestTUI_RenderToLines(t *testing.T) {
	text := NewText("hello")
	tui := New(text)

	lines := tui.RenderToLines(80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "hello" {
		t.Errorf("expected 'hello', got %q", lines[0])
	}
}

func TestTUI_RenderToLines_MultipleComponents(t *testing.T) {
	t1 := NewText("line one")
	t2 := NewText("line two")
	c := NewContainer(t1, t2)
	tui := New(c)

	lines := tui.RenderToLines(80)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "line one" {
		t.Errorf("expected 'line one', got %q", lines[0])
	}
	if lines[1] != "line two" {
		t.Errorf("expected 'line two', got %q", lines[1])
	}
}

func TestTUI_RenderToLines_NilRoot(t *testing.T) {
	tui := New(nil)
	lines := tui.RenderToLines(80)
	if lines != nil {
		t.Errorf("expected nil for nil root, got %v", lines)
	}
}

func TestTUI_SetRoot(t *testing.T) {
	t1 := NewText("first")
	tui := New(t1)

	lines := tui.RenderToLines(80)
	if lines[0] != "first" {
		t.Errorf("expected 'first', got %q", lines[0])
	}

	t2 := NewText("second")
	tui.SetRoot(t2)

	lines = tui.RenderToLines(80)
	if len(lines) != 1 || lines[0] != "second" {
		t.Errorf("expected 'second' after SetRoot, got %v", lines)
	}
}

func TestTUI_Root(t *testing.T) {
	text := NewText("root")
	tui := New(text)

	root := tui.Root()
	if root != text {
		t.Error("Root() should return the root component")
	}
}

func TestTUI_SetFocus(t *testing.T) {
	inp1 := NewInput()
	inp2 := NewInput()
	c := NewContainer(inp1, inp2)
	tui := New(c)

	tui.SetFocus(inp1)
	if !inp1.IsFocused() {
		t.Error("inp1 should be focused after SetFocus")
	}
	if inp2.IsFocused() {
		t.Error("inp2 should not be focused")
	}
	if tui.Focus() != inp1 {
		t.Error("Focus() should return inp1")
	}

	tui.SetFocus(inp2)
	if inp1.IsFocused() {
		t.Error("inp1 should lose focus when inp2 gains focus")
	}
	if !inp2.IsFocused() {
		t.Error("inp2 should be focused")
	}
	if tui.Focus() != inp2 {
		t.Error("Focus() should return inp2")
	}
}

func TestTUI_SetFocus_Nil(t *testing.T) {
	inp := NewInput()
	tui := New(NewContainer(inp))

	tui.SetFocus(inp)
	if !inp.IsFocused() {
		t.Error("inp should be focused")
	}

	tui.SetFocus(nil)
	if inp.IsFocused() {
		t.Error("inp should lose focus when SetFocus(nil)")
	}
	if tui.Focus() != nil {
		t.Error("Focus() should return nil after SetFocus(nil)")
	}
}

func TestTUI_SetRoot_ClearsLastLines(t *testing.T) {
	t1 := NewText("first")
	tui := New(t1)

	// Render once to populate lastLines.
	tui.RenderToLines(80)

	t2 := NewText("second")
	tui.SetRoot(t2)

	// After SetRoot, lastLines should be cleared (nil).
	// Verify by rendering - should work fine.
	lines := tui.RenderToLines(80)
	if lines[0] != "second" {
		t.Errorf("expected 'second', got %q", lines[0])
	}
}

func TestTUI_ComplexComponentTree(t *testing.T) {
	header := NewText("Header")
	body := NewText("Body text here")
	spacer := NewSpacer(1)
	footer := NewText("Footer")

	root := NewContainer(header, spacer, body, spacer, footer)
	tui := New(root)

	lines := tui.RenderToLines(80)
	// header (1) + spacer (1) + body (1) + spacer (1) + footer (1) = 5
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "Header" {
		t.Errorf("expected 'Header', got %q", lines[0])
	}
	if lines[1] != "" {
		t.Errorf("expected empty spacer line, got %q", lines[1])
	}
	if lines[2] != "Body text here" {
		t.Errorf("expected 'Body text here', got %q", lines[2])
	}
	if lines[4] != "Footer" {
		t.Errorf("expected 'Footer', got %q", lines[4])
	}
}

func TestTUI_RenderToLines_WrapsContent(t *testing.T) {
	text := NewText("this is a long line that should wrap when rendered at a narrow width")
	tui := New(text)

	lines := tui.RenderToLines(20)
	if len(lines) < 2 {
		t.Errorf("expected wrapped lines at width 20, got %d lines", len(lines))
	}
}

func TestTUI_SetWriter(t *testing.T) {
	// Just verify it doesn't panic.
	tui := New(NewText("test"))
	var buf mockWriter
	tui.SetWriter(&buf)
}

type mockWriter struct {
	data []byte
}

func (m *mockWriter) Write(p []byte) (int, error) {
	m.data = append(m.data, p...)
	return len(p), nil
}

func TestTUI_ShowOverlay(t *testing.T) {
	root := NewText("background")
	tui := New(root)

	overlay := NewText("overlay")
	removeOverlay := tui.ShowOverlay(overlay, OverlayOptions{
		Position: OverlayCenter,
	})

	if removeOverlay == nil {
		t.Fatal("ShowOverlay should return a remove function")
	}

	// Calling remove should not panic.
	removeOverlay()
}

func TestTUI_ClearOverlays(t *testing.T) {
	root := NewText("background")
	tui := New(root)

	tui.ShowOverlay(NewText("ov1"), OverlayOptions{})
	tui.ShowOverlay(NewText("ov2"), OverlayOptions{})

	tui.ClearOverlays()
	// Should not panic and overlays should be cleared.
}

func TestTUI_SetOnRender(t *testing.T) {
	tui := New(NewText("test"))
	called := false
	tui.SetOnRender(func() {
		called = true
	})
	// Note: onRender is called during Render(), which requires width/height > 0.
	// RenderToLines doesn't call onRender, but this verifies SetOnRender doesn't panic.
	if called {
		t.Error("onRender should not be called yet")
	}
}

func TestTUI_SetOnInput(t *testing.T) {
	tui := New(NewText("test"))
	// Just verify it doesn't panic.
	tui.SetOnInput(func(data []byte) {})
}

func TestTUI_Size(t *testing.T) {
	tui := New(NewText("test"))
	w, h := tui.Size()
	// By default, width and height are 0 since updateSize hasn't been called.
	if w != 0 || h != 0 {
		t.Errorf("initial size should be (0,0), got (%d,%d)", w, h)
	}
}
