package tui

import (
	"strings"
	"testing"
)

func TestBox_RenderChildWithPadding(t *testing.T) {
	child := NewText("content")
	box := NewBox(child)
	box.SetPadding(Padding{Top: 1, Right: 2, Bottom: 1, Left: 2})

	lines := box.Render(40)

	// Top padding (1 line) + content (1 line) + bottom padding (1 line) = 3 lines.
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (top pad + content + bottom pad), got %d: %v", len(lines), lines)
	}

	// The content line should contain "content".
	found := false
	for _, line := range lines {
		if strings.Contains(StripAnsi(line), "content") {
			found = true
			break
		}
	}
	if !found {
		t.Error("box should contain child content")
	}
}

func TestBox_WithBorder(t *testing.T) {
	child := NewText("inside")
	box := NewBox(child)
	box.SetBorder(BorderSingle)

	lines := box.Render(30)

	// Should have: top border + content + bottom border = 3 lines minimum.
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (borders + content), got %d: %v", len(lines), lines)
	}

	// Top border should start with corner.
	stripped := StripAnsi(lines[0])
	if !strings.HasPrefix(stripped, "\u250c") {
		t.Errorf("top border should start with top-left corner, got %q", stripped)
	}
	if !strings.HasSuffix(stripped, "\u2510") {
		t.Errorf("top border should end with top-right corner, got %q", stripped)
	}

	// Bottom border.
	last := StripAnsi(lines[len(lines)-1])
	if !strings.HasPrefix(last, "\u2514") {
		t.Errorf("bottom border should start with bottom-left corner, got %q", last)
	}
	if !strings.HasSuffix(last, "\u2518") {
		t.Errorf("bottom border should end with bottom-right corner, got %q", last)
	}

	// Content line should have vertical borders.
	contentLine := StripAnsi(lines[1])
	if !strings.HasPrefix(contentLine, "\u2502") {
		t.Errorf("content line should start with vertical border, got %q", contentLine)
	}
}

func TestBox_UniformPadding(t *testing.T) {
	child := NewText("padded")
	box := NewBox(child)
	box.SetPadding(UniformPadding(2))

	lines := box.Render(40)

	// Top padding (2) + content (1) + bottom padding (2) = 5 lines.
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines with uniform padding 2, got %d: %v", len(lines), lines)
	}

	// Padding lines should be spaces.
	for _, idx := range []int{0, 1, 3, 4} {
		stripped := StripAnsi(lines[idx])
		if strings.TrimSpace(stripped) != "" {
			t.Errorf("padding line %d should be empty/spaces, got %q", idx, stripped)
		}
	}
}

func TestBox_HVPadding(t *testing.T) {
	p := HVPadding(3, 1)
	if p.Left != 3 || p.Right != 3 {
		t.Errorf("HVPadding horizontal should be 3, got left=%d right=%d", p.Left, p.Right)
	}
	if p.Top != 1 || p.Bottom != 1 {
		t.Errorf("HVPadding vertical should be 1, got top=%d bottom=%d", p.Top, p.Bottom)
	}
}

func TestBox_NoChild(t *testing.T) {
	box := NewBox(nil)
	box.SetPadding(Padding{Top: 1, Bottom: 1})

	lines := box.Render(20)

	// With no child: top pad (1) + bottom pad (1) = 2 lines.
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines for nil child with padding, got %d: %v", len(lines), lines)
	}
}

func TestBox_NoChildWithBorder(t *testing.T) {
	box := NewBox(nil)
	box.SetBorder(BorderSingle)

	lines := box.Render(20)

	// top border + bottom border = 2.
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines (borders), got %d: %v", len(lines), lines)
	}
}

func TestBox_BorderStyleSingle(t *testing.T) {
	child := NewText("x")
	box := NewBox(child)
	box.SetBorder(BorderSingle)

	lines := box.Render(20)
	top := StripAnsi(lines[0])
	if !strings.Contains(top, "\u2500") {
		t.Errorf("single border should use thin horizontal line, got %q", top)
	}
}

func TestBox_BorderStyleDouble(t *testing.T) {
	child := NewText("x")
	box := NewBox(child)
	box.SetBorder(BorderDouble)

	lines := box.Render(20)
	top := StripAnsi(lines[0])
	if !strings.Contains(top, "\u2550") {
		t.Errorf("double border should use double horizontal line, got %q", top)
	}
	if !strings.HasPrefix(top, "\u2554") {
		t.Errorf("double border should start with double top-left corner, got %q", top)
	}
}

func TestBox_BorderStyleRounded(t *testing.T) {
	child := NewText("x")
	box := NewBox(child)
	box.SetBorder(BorderRounded)

	lines := box.Render(20)
	top := StripAnsi(lines[0])
	if !strings.HasPrefix(top, "\u256d") {
		t.Errorf("rounded border should start with rounded top-left corner, got %q", top)
	}
	if !strings.HasSuffix(top, "\u256e") {
		t.Errorf("rounded border should end with rounded top-right corner, got %q", top)
	}
}

func TestBox_BorderStyleHeavy(t *testing.T) {
	child := NewText("x")
	box := NewBox(child)
	box.SetBorder(BorderHeavy)

	lines := box.Render(20)
	top := StripAnsi(lines[0])
	if !strings.HasPrefix(top, "\u250f") {
		t.Errorf("heavy border should start with heavy top-left corner, got %q", top)
	}
	if !strings.Contains(top, "\u2501") {
		t.Errorf("heavy border should use heavy horizontal line, got %q", top)
	}
}

func TestBox_BorderNone(t *testing.T) {
	child := NewText("text")
	box := NewBox(child)
	box.SetBorder(BorderNone)

	lines := box.Render(20)
	// No border, just the content.
	if len(lines) != 1 {
		t.Fatalf("expected 1 line with no border/padding, got %d", len(lines))
	}
}

func TestBox_Invalidate(t *testing.T) {
	child := NewText("child")
	box := NewBox(child)

	box.Render(40)
	if box.IsDirty() {
		t.Fatal("box should not be dirty after render")
	}

	box.Invalidate()
	if !box.IsDirty() {
		t.Error("box should be dirty after Invalidate")
	}
}

func TestBox_SetChild(t *testing.T) {
	child1 := NewText("first")
	child2 := NewText("second")
	box := NewBox(child1)

	lines := box.Render(40)
	if !strings.Contains(StripAnsi(lines[0]), "first") {
		t.Error("should render first child")
	}

	box.SetChild(child2)
	lines = box.Render(40)
	if !strings.Contains(StripAnsi(lines[0]), "second") {
		t.Error("should render second child after SetChild")
	}
}

func TestBox_SetBorderColor(t *testing.T) {
	child := NewText("x")
	box := NewBox(child)
	box.SetBorder(BorderSingle)
	box.SetBorderColor(FgRed)

	lines := box.Render(20)
	// Top border should contain the color.
	if !strings.Contains(lines[0], FgRed) {
		t.Errorf("border should use specified color: %q", lines[0])
	}
}

func TestBox_SetBackground(t *testing.T) {
	child := NewText("x")
	box := NewBox(child)
	box.SetBackground(BgBlue)

	lines := box.Render(20)
	if !strings.Contains(lines[0], BgBlue) {
		t.Errorf("box should use specified background: %q", lines[0])
	}
}

func TestBox_WithBorderAndPadding(t *testing.T) {
	child := NewText("hello")
	box := NewBox(child)
	box.SetBorder(BorderSingle)
	box.SetPadding(Padding{Top: 1, Right: 1, Bottom: 1, Left: 1})

	lines := box.Render(30)
	// border top + padding top + content + padding bottom + border bottom = 5.
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %v", len(lines), lines)
	}
}
