package tui

import (
	"strings"
	"testing"
)

func makeItems(labels ...string) []SelectItem {
	items := make([]SelectItem, len(labels))
	for i, l := range labels {
		items[i] = SelectItem{Label: l, Value: l}
	}
	return items
}

func TestSelectList_RendersItems(t *testing.T) {
	items := makeItems("Apple", "Banana", "Cherry")
	sl := NewSelectList(items)
	sl.SetFocused(true)

	lines := sl.Render(40)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines for 3 items, got %d: %v", len(lines), lines)
	}

	// Check that all items appear.
	allText := ""
	for _, line := range lines {
		allText += StripAnsi(line) + "\n"
	}
	for _, item := range items {
		if !strings.Contains(allText, item.Label) {
			t.Errorf("expected item %q in output", item.Label)
		}
	}
}

func TestSelectList_HandleInput_UpDown(t *testing.T) {
	items := makeItems("A", "B", "C")
	sl := NewSelectList(items)
	sl.SetFocused(true)

	// Initial selection should be index 0.
	_, idx := sl.SelectedItem()
	if idx != 0 {
		t.Errorf("initial selection should be 0, got %d", idx)
	}

	// Move down.
	consumed := sl.HandleInput([]byte{0x1b, '[', 'B'})
	if !consumed {
		t.Error("down arrow should be consumed")
	}
	_, idx = sl.SelectedItem()
	if idx != 1 {
		t.Errorf("expected selection 1 after down, got %d", idx)
	}

	// Move down again.
	sl.HandleInput([]byte{0x1b, '[', 'B'})
	_, idx = sl.SelectedItem()
	if idx != 2 {
		t.Errorf("expected selection 2 after second down, got %d", idx)
	}

	// Move down at bottom - should stay at 2.
	sl.HandleInput([]byte{0x1b, '[', 'B'})
	_, idx = sl.SelectedItem()
	if idx != 2 {
		t.Errorf("selection should stay at 2 at bottom, got %d", idx)
	}

	// Move up.
	sl.HandleInput([]byte{0x1b, '[', 'A'})
	_, idx = sl.SelectedItem()
	if idx != 1 {
		t.Errorf("expected selection 1 after up, got %d", idx)
	}
}

func TestSelectList_HandleInput_JK(t *testing.T) {
	items := makeItems("A", "B", "C")
	sl := NewSelectList(items)
	sl.SetFocused(true)

	// j moves down.
	consumed := sl.HandleInput([]byte{'j'})
	if !consumed {
		t.Error("j should be consumed")
	}
	_, idx := sl.SelectedItem()
	if idx != 1 {
		t.Errorf("expected selection 1 after j, got %d", idx)
	}

	// k moves up.
	consumed = sl.HandleInput([]byte{'k'})
	if !consumed {
		t.Error("k should be consumed")
	}
	_, idx = sl.SelectedItem()
	if idx != 0 {
		t.Errorf("expected selection 0 after k, got %d", idx)
	}
}

func TestSelectList_HighlightsSelected(t *testing.T) {
	items := makeItems("A", "B", "C")
	sl := NewSelectList(items)
	sl.SetFocused(true)

	lines := sl.Render(40)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// First item (selected) should have the cursor indicator.
	stripped0 := StripAnsi(lines[0])
	if !strings.Contains(stripped0, "\u25b8") {
		t.Errorf("selected item should have cursor indicator, got %q", stripped0)
	}

	// Second item should not have cursor.
	stripped1 := StripAnsi(lines[1])
	if strings.Contains(stripped1, "\u25b8") {
		t.Errorf("non-selected item should not have cursor: %q", stripped1)
	}
}

func TestSelectList_HandleInput_Enter(t *testing.T) {
	items := makeItems("A", "B", "C")
	sl := NewSelectList(items)
	sl.SetFocused(true)

	var selectedItem SelectItem
	var selectedIdx int
	sl.SetOnSelect(func(item SelectItem, idx int) {
		selectedItem = item
		selectedIdx = idx
	})

	// Move to second item and select.
	sl.HandleInput([]byte{0x1b, '[', 'B'}) // Down
	consumed := sl.HandleInput([]byte{'\r'}) // Enter
	if !consumed {
		t.Error("enter should be consumed")
	}
	if selectedItem.Label != "B" {
		t.Errorf("expected 'B' selected, got %q", selectedItem.Label)
	}
	if selectedIdx != 1 {
		t.Errorf("expected index 1, got %d", selectedIdx)
	}
}

func TestSelectList_HandleInput_EnterNoCallback(t *testing.T) {
	items := makeItems("A")
	sl := NewSelectList(items)
	sl.SetFocused(true)

	// Should not panic.
	consumed := sl.HandleInput([]byte{'\r'})
	if !consumed {
		t.Error("enter should be consumed even without callback")
	}
}

func TestSelectList_Scrolling(t *testing.T) {
	items := makeItems("1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12")
	sl := NewSelectList(items)
	sl.SetMaxVisible(5)
	sl.SetFocused(true)

	// Render should show maxVisible items plus potential scroll indicators.
	lines := sl.Render(40)
	// At least 5 item lines.
	nonEmpty := 0
	for _, line := range lines {
		if StripAnsi(line) != "" {
			nonEmpty++
		}
	}
	if nonEmpty < 5 {
		t.Errorf("expected at least 5 non-empty lines, got %d", nonEmpty)
	}

	// Move down past visible area.
	for i := 0; i < 6; i++ {
		sl.HandleInput([]byte{0x1b, '[', 'B'})
	}

	lines = sl.Render(40)
	allText := ""
	for _, line := range lines {
		allText += StripAnsi(line) + "\n"
	}

	// Should show scroll indicator.
	if !strings.Contains(allText, "\u2191") {
		t.Errorf("expected up scroll indicator when scrolled down")
	}
}

func TestSelectList_OnChange(t *testing.T) {
	items := makeItems("A", "B", "C")
	sl := NewSelectList(items)
	sl.SetFocused(true)

	var changedItems []string
	sl.SetOnChange(func(item SelectItem, idx int) {
		changedItems = append(changedItems, item.Label)
	})

	sl.HandleInput([]byte{0x1b, '[', 'B'}) // Down
	sl.HandleInput([]byte{0x1b, '[', 'B'}) // Down again

	if len(changedItems) != 2 {
		t.Fatalf("expected 2 onChange calls, got %d", len(changedItems))
	}
	if changedItems[0] != "B" {
		t.Errorf("first change should be 'B', got %q", changedItems[0])
	}
	if changedItems[1] != "C" {
		t.Errorf("second change should be 'C', got %q", changedItems[1])
	}
}

func TestSelectList_SelectedItem(t *testing.T) {
	items := makeItems("X", "Y", "Z")
	sl := NewSelectList(items)

	item, idx := sl.SelectedItem()
	if item.Label != "X" || idx != 0 {
		t.Errorf("expected 'X' at 0, got %q at %d", item.Label, idx)
	}
}

func TestSelectList_SelectedIndex(t *testing.T) {
	items := makeItems("A", "B")
	sl := NewSelectList(items)

	if sl.SelectedIndex() != 0 {
		t.Errorf("expected SelectedIndex 0, got %d", sl.SelectedIndex())
	}

	sl.HandleInput([]byte{0x1b, '[', 'B'}) // Down
	if sl.SelectedIndex() != 1 {
		t.Errorf("expected SelectedIndex 1, got %d", sl.SelectedIndex())
	}
}

func TestSelectList_EmptyItems(t *testing.T) {
	sl := NewSelectList([]SelectItem{})
	sl.SetFocused(true)

	item, idx := sl.SelectedItem()
	if idx != -1 {
		t.Errorf("expected -1 for empty list, got %d", idx)
	}
	if item.Label != "" {
		t.Errorf("expected empty item, got %q", item.Label)
	}

	lines := sl.Render(40)
	if len(lines) == 0 {
		t.Fatal("empty list should still render something")
	}
}

func TestSelectList_SetItems(t *testing.T) {
	sl := NewSelectList(makeItems("A"))
	sl.HandleInput([]byte{0x1b, '[', 'B'}) // This won't change since only 1 item.

	sl.SetItems(makeItems("X", "Y", "Z"))
	if sl.SelectedIndex() != 0 {
		t.Errorf("SetItems should reset selection to 0, got %d", sl.SelectedIndex())
	}
	item, _ := sl.SelectedItem()
	if item.Label != "X" {
		t.Errorf("expected 'X' after SetItems, got %q", item.Label)
	}
}

func TestSelectList_Focus(t *testing.T) {
	sl := NewSelectList(makeItems("A"))
	if sl.IsFocused() {
		t.Error("should not be focused by default")
	}
	sl.SetFocused(true)
	if !sl.IsFocused() {
		t.Error("should be focused after SetFocused(true)")
	}
}

func TestSelectList_HomeEnd(t *testing.T) {
	items := makeItems("A", "B", "C", "D", "E")
	sl := NewSelectList(items)
	sl.SetFocused(true)

	// Move down a few times.
	sl.HandleInput([]byte{0x1b, '[', 'B'})
	sl.HandleInput([]byte{0x1b, '[', 'B'})

	// Home.
	sl.HandleInput([]byte("\x1b[H"))
	if sl.SelectedIndex() != 0 {
		t.Errorf("Home should select first item, got %d", sl.SelectedIndex())
	}

	// End.
	sl.HandleInput([]byte("\x1b[F"))
	if sl.SelectedIndex() != 4 {
		t.Errorf("End should select last item, got %d", sl.SelectedIndex())
	}
}

func TestSelectList_ZeroWidth(t *testing.T) {
	sl := NewSelectList(makeItems("A", "B"))
	lines := sl.Render(0)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
}

func TestSelectList_SetCursor(t *testing.T) {
	sl := NewSelectList(makeItems("A", "B"))
	sl.SetCursor("> ")
	sl.SetFocused(true)

	lines := sl.Render(40)
	stripped := StripAnsi(lines[0])
	if !strings.Contains(stripped, ">") {
		t.Errorf("custom cursor should appear, got %q", stripped)
	}
}

func TestSelectList_CtrlP_CtrlN(t *testing.T) {
	items := makeItems("A", "B", "C")
	sl := NewSelectList(items)
	sl.SetFocused(true)

	// Ctrl+N moves down.
	sl.HandleInput([]byte{0x0e})
	if sl.SelectedIndex() != 1 {
		t.Errorf("Ctrl+N should move down, got %d", sl.SelectedIndex())
	}

	// Ctrl+P moves up.
	sl.HandleInput([]byte{0x10})
	if sl.SelectedIndex() != 0 {
		t.Errorf("Ctrl+P should move up, got %d", sl.SelectedIndex())
	}
}

func TestSelectList_FilterMode(t *testing.T) {
	items := makeItems("Apple", "Banana", "Cherry")
	sl := NewSelectList(items)
	sl.SetFocused(true)
	sl.SetShowFilter(true)

	// Enter filter mode with '/'.
	sl.HandleInput([]byte{'/'})

	// Type filter text.
	sl.HandleInput([]byte{'b'})
	sl.HandleInput([]byte{'a'})

	// Only "Banana" should match.
	lines := sl.Render(40)
	allText := ""
	for _, line := range lines {
		allText += StripAnsi(line) + "\n"
	}
	if !strings.Contains(allText, "Banana") {
		t.Errorf("filtered list should contain 'Banana'")
	}
	if strings.Contains(allText, "Apple") {
		t.Errorf("filtered list should not contain 'Apple'")
	}
	if strings.Contains(allText, "Cherry") {
		t.Errorf("filtered list should not contain 'Cherry'")
	}

	// Escape exits filter mode.
	sl.HandleInput([]byte{0x1b})
}
