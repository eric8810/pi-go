package tui

import (
	"testing"
)

func TestBaseComponent_DirtyFlag(t *testing.T) {
	t.Run("starts dirty when initialized with dirty true", func(t *testing.T) {
		bc := &BaseComponent{dirty: true}
		if !bc.IsDirty() {
			t.Error("BaseComponent should start dirty when initialized with dirty: true")
		}
	})

	t.Run("zero value starts not dirty", func(t *testing.T) {
		bc := &BaseComponent{}
		if bc.IsDirty() {
			t.Error("zero-value BaseComponent should not be dirty")
		}
	})
}

func TestBaseComponent_Invalidate(t *testing.T) {
	bc := &BaseComponent{}
	// Set some cached data so it's not dirty.
	bc.SetCached(80, []string{"line"})
	if bc.IsDirty() {
		t.Fatal("should not be dirty after SetCached")
	}

	bc.Invalidate()
	if !bc.IsDirty() {
		t.Error("Invalidate should set dirty flag")
	}
}

func TestBaseComponent_GetCached(t *testing.T) {
	t.Run("returns nil when dirty", func(t *testing.T) {
		bc := &BaseComponent{dirty: true}
		result := bc.GetCached(80)
		if result != nil {
			t.Errorf("GetCached should return nil when dirty, got %v", result)
		}
	})

	t.Run("returns cached when clean and width matches", func(t *testing.T) {
		bc := &BaseComponent{}
		lines := []string{"hello", "world"}
		bc.SetCached(80, lines)

		result := bc.GetCached(80)
		if result == nil {
			t.Fatal("GetCached should return cached lines when clean")
		}
		if len(result) != 2 || result[0] != "hello" || result[1] != "world" {
			t.Errorf("GetCached returned unexpected content: %v", result)
		}
	})

	t.Run("returns nil when width changes", func(t *testing.T) {
		bc := &BaseComponent{}
		bc.SetCached(80, []string{"line"})

		result := bc.GetCached(100)
		if result != nil {
			t.Errorf("GetCached should return nil on width change, got %v", result)
		}
	})

	t.Run("returns copy not reference", func(t *testing.T) {
		bc := &BaseComponent{}
		bc.SetCached(80, []string{"hello"})

		result := bc.GetCached(80)
		result[0] = "modified"

		result2 := bc.GetCached(80)
		if result2[0] != "hello" {
			t.Error("GetCached should return a copy, not a reference")
		}
	})
}

func TestContainer_RenderVertically(t *testing.T) {
	t1 := NewText("line one")
	t2 := NewText("line two")
	c := NewContainer(t1, t2)

	lines := c.Render(80)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "line one" {
		t.Errorf("first line should be 'line one', got %q", lines[0])
	}
	if lines[1] != "line two" {
		t.Errorf("second line should be 'line two', got %q", lines[1])
	}
}

func TestContainer_AddChild(t *testing.T) {
	c := NewContainer()
	if len(c.Children()) != 0 {
		t.Fatalf("expected 0 children, got %d", len(c.Children()))
	}

	t1 := NewText("child 1")
	c.AddChild(t1)
	if len(c.Children()) != 1 {
		t.Fatalf("expected 1 child, got %d", len(c.Children()))
	}

	t2 := NewText("child 2")
	c.AddChild(t2)
	if len(c.Children()) != 2 {
		t.Fatalf("expected 2 children, got %d", len(c.Children()))
	}

	lines := c.Render(80)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestContainer_RemoveChild(t *testing.T) {
	t1 := NewText("child 1")
	t2 := NewText("child 2")
	t3 := NewText("child 3")
	c := NewContainer(t1, t2, t3)

	c.RemoveChild(t2)
	if len(c.Children()) != 2 {
		t.Fatalf("expected 2 children after remove, got %d", len(c.Children()))
	}

	lines := c.Render(80)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "child 1" || lines[1] != "child 3" {
		t.Errorf("unexpected lines after RemoveChild: %v", lines)
	}
}

func TestContainer_RemoveChildNotPresent(t *testing.T) {
	t1 := NewText("child 1")
	t2 := NewText("not added")
	c := NewContainer(t1)

	// Removing a child that isn't present should not panic.
	c.RemoveChild(t2)
	if len(c.Children()) != 1 {
		t.Fatalf("expected 1 child, got %d", len(c.Children()))
	}
}

func TestContainer_SetChildren(t *testing.T) {
	t1 := NewText("old")
	c := NewContainer(t1)

	t2 := NewText("new1")
	t3 := NewText("new2")
	c.SetChildren([]Component{t2, t3})

	if len(c.Children()) != 2 {
		t.Fatalf("expected 2 children, got %d", len(c.Children()))
	}

	lines := c.Render(80)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "new1" || lines[1] != "new2" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestContainer_NoChildren(t *testing.T) {
	c := NewContainer()
	lines := c.Render(80)
	// With no children, should produce nil/empty lines.
	if len(lines) != 0 {
		t.Errorf("expected 0 lines for empty container, got %d: %v", len(lines), lines)
	}
}

func TestContainer_FindFocusable(t *testing.T) {
	t.Run("returns first focused focusable", func(t *testing.T) {
		inp1 := NewInput()
		inp1.SetFocused(false)
		inp2 := NewInput()
		inp2.SetFocused(true)
		text := NewText("plain")

		c := NewContainer(inp1, text, inp2)

		f := c.FindFocusable()
		if f == nil {
			t.Fatal("FindFocusable should return a focused component")
		}
		if f != inp2 {
			t.Error("FindFocusable should return inp2 (the focused input)")
		}
	})

	t.Run("returns nil when no focusable is focused", func(t *testing.T) {
		inp := NewInput()
		inp.SetFocused(false)
		text := NewText("text")
		c := NewContainer(inp, text)

		f := c.FindFocusable()
		if f != nil {
			t.Error("FindFocusable should return nil when nothing is focused")
		}
	})

	t.Run("searches nested containers", func(t *testing.T) {
		inp := NewInput()
		inp.SetFocused(true)
		inner := NewContainer(inp)
		outer := NewContainer(inner)

		f := outer.FindFocusable()
		if f != inp {
			t.Error("FindFocusable should search nested containers")
		}
	})
}

func TestContainer_AllFocusables(t *testing.T) {
	inp1 := NewInput()
	inp2 := NewInput()
	text := NewText("not focusable")
	inner := NewContainer(inp2)
	c := NewContainer(inp1, text, inner)

	focusables := c.AllFocusables()
	if len(focusables) != 2 {
		t.Fatalf("expected 2 focusables, got %d", len(focusables))
	}
}

func TestContainer_Invalidate(t *testing.T) {
	t1 := NewText("child")
	c := NewContainer(t1)

	// Render to cache.
	c.Render(80)
	if c.IsDirty() {
		t.Fatal("container should not be dirty after render")
	}

	c.Invalidate()
	if !c.IsDirty() {
		t.Error("container should be dirty after Invalidate")
	}
}

func TestContainer_Caching(t *testing.T) {
	t1 := NewText("cached")
	c := NewContainer(t1)

	lines1 := c.Render(80)
	lines2 := c.Render(80)

	if len(lines1) != len(lines2) {
		t.Fatalf("cached render should produce same result")
	}
	for i := range lines1 {
		if lines1[i] != lines2[i] {
			t.Errorf("line %d differs: %q vs %q", i, lines1[i], lines2[i])
		}
	}
}
