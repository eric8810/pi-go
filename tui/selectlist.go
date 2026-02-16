package tui

import (
	"strings"
)

// SelectItem represents a single item in a SelectList.
type SelectItem struct {
	Label string // Display text for the item.
	Value string // Value returned when selected.
}

// SelectList is an interactive selection list component that supports
// navigation, filtering, and selection highlighting.
type SelectList struct {
	BaseComponent
	focused      bool
	items        []SelectItem
	filtered     []int // Indices into items that match the filter.
	selectedIdx  int   // Index into filtered.
	scrollTop    int
	maxVisible   int
	filterText   string
	filterMode   bool
	showFilter   bool // Whether to show a filter input.
	onSelect     func(item SelectItem, index int)
	onChange     func(item SelectItem, index int)
	cursor       string // Cursor indicator for selected item.
	activeStyle  string // ANSI style for the selected item.
}

// NewSelectList creates a new SelectList with the given items.
func NewSelectList(items []SelectItem) *SelectList {
	sl := &SelectList{
		BaseComponent: BaseComponent{dirty: true},
		items:         items,
		maxVisible:    10,
		cursor:        "▸ ",
		activeStyle:   BoldOn + FgCyan,
	}
	sl.rebuildFiltered()
	return sl
}

// SetItems replaces the item list and resets selection.
func (sl *SelectList) SetItems(items []SelectItem) {
	sl.items = items
	sl.selectedIdx = 0
	sl.scrollTop = 0
	sl.filterText = ""
	sl.rebuildFiltered()
	sl.MarkDirty()
}

// SetMaxVisible sets the maximum number of visible items.
func (sl *SelectList) SetMaxVisible(n int) {
	sl.maxVisible = n
	sl.MarkDirty()
}

// SetShowFilter enables or disables the filter input line.
func (sl *SelectList) SetShowFilter(show bool) {
	sl.showFilter = show
	sl.MarkDirty()
}

// SetOnSelect sets a callback invoked when an item is selected (Enter pressed).
func (sl *SelectList) SetOnSelect(fn func(item SelectItem, index int)) {
	sl.onSelect = fn
}

// SetOnChange sets a callback invoked when the highlighted item changes.
func (sl *SelectList) SetOnChange(fn func(item SelectItem, index int)) {
	sl.onChange = fn
}

// SetCursor sets the cursor indicator string shown before the selected item.
func (sl *SelectList) SetCursor(cursor string) {
	sl.cursor = cursor
	sl.MarkDirty()
}

// SetActiveStyle sets the ANSI style applied to the selected item.
func (sl *SelectList) SetActiveStyle(style string) {
	sl.activeStyle = style
	sl.MarkDirty()
}

// SelectedItem returns the currently highlighted item and its original index
// in the items slice. Returns an empty SelectItem and -1 if no items match.
func (sl *SelectList) SelectedItem() (SelectItem, int) {
	if len(sl.filtered) == 0 {
		return SelectItem{}, -1
	}
	origIdx := sl.filtered[sl.selectedIdx]
	return sl.items[origIdx], origIdx
}

// SelectedIndex returns the index of the currently highlighted item in the
// original items slice. Returns -1 if no items match the filter.
func (sl *SelectList) SelectedIndex() int {
	if len(sl.filtered) == 0 {
		return -1
	}
	return sl.filtered[sl.selectedIdx]
}

// SetFocused sets the focus state.
func (sl *SelectList) SetFocused(focused bool) {
	sl.focused = focused
	sl.MarkDirty()
}

// IsFocused returns whether this list currently has focus.
func (sl *SelectList) IsFocused() bool {
	return sl.focused
}

// rebuildFiltered rebuilds the filtered index list based on filterText.
func (sl *SelectList) rebuildFiltered() {
	sl.filtered = nil
	filter := strings.ToLower(sl.filterText)
	for i, item := range sl.items {
		if filter == "" || strings.Contains(strings.ToLower(item.Label), filter) {
			sl.filtered = append(sl.filtered, i)
		}
	}
	if sl.selectedIdx >= len(sl.filtered) {
		sl.selectedIdx = len(sl.filtered) - 1
	}
	if sl.selectedIdx < 0 {
		sl.selectedIdx = 0
	}
}

// ensureVisible adjusts scrollTop so the selected item is visible.
func (sl *SelectList) ensureVisible() {
	if sl.maxVisible <= 0 {
		sl.scrollTop = 0
		return
	}
	if sl.selectedIdx < sl.scrollTop {
		sl.scrollTop = sl.selectedIdx
	}
	if sl.selectedIdx >= sl.scrollTop+sl.maxVisible {
		sl.scrollTop = sl.selectedIdx - sl.maxVisible + 1
	}
}

// Render produces the lines of the selection list.
func (sl *SelectList) Render(width int) []string {
	if width <= 0 {
		return []string{""}
	}

	sl.ensureVisible()

	var result []string

	// Filter input line.
	if sl.showFilter {
		filterLine := DimOn + "Filter: " + DimOff + sl.filterText
		if sl.focused && sl.filterMode {
			filterLine += InverseOn + " " + InverseOff
		}
		result = append(result, TruncateToWidth(filterLine, width))
	}

	if len(sl.filtered) == 0 {
		result = append(result, DimOn+"  (no matching items)"+DimOff)
		return result
	}

	visCount := len(sl.filtered)
	if sl.maxVisible > 0 && visCount > sl.maxVisible {
		visCount = sl.maxVisible
	}

	endIdx := sl.scrollTop + visCount
	if endIdx > len(sl.filtered) {
		endIdx = len(sl.filtered)
	}

	cursorWidth := VisibleWidth(sl.cursor)
	blankCursor := strings.Repeat(" ", cursorWidth)

	for i := sl.scrollTop; i < endIdx; i++ {
		origIdx := sl.filtered[i]
		item := sl.items[origIdx]

		var line string
		if i == sl.selectedIdx {
			label := item.Label
			maxLabelW := width - cursorWidth
			if maxLabelW > 0 && VisibleWidth(label) > maxLabelW {
				label = TruncateToWidth(label, maxLabelW)
			}
			line = sl.activeStyle + sl.cursor + label + Reset
		} else {
			label := item.Label
			maxLabelW := width - cursorWidth
			if maxLabelW > 0 && VisibleWidth(label) > maxLabelW {
				label = TruncateToWidth(label, maxLabelW)
			}
			line = blankCursor + label
		}

		result = append(result, line)
	}

	// Scroll indicators.
	if sl.scrollTop > 0 {
		// There are items above.
		indicator := DimOn + "  ↑ " + itoa(sl.scrollTop) + " more" + DimOff
		result = append([]string{indicator}, result...)
	}
	if endIdx < len(sl.filtered) {
		remaining := len(sl.filtered) - endIdx
		indicator := DimOn + "  ↓ " + itoa(remaining) + " more" + DimOff
		result = append(result, indicator)
	}

	return result
}

// HandleInput processes keyboard input for the selection list.
func (sl *SelectList) HandleInput(data []byte) bool {
	if sl.filterMode && sl.showFilter {
		return sl.handleFilterInput(data)
	}

	switch {
	case MatchesKey(data, KeyUp), MatchesKey(data, KeyCtrlP):
		sl.moveUp()
		return true

	case MatchesKey(data, KeyDown), MatchesKey(data, KeyCtrlN):
		sl.moveDown()
		return true

	case MatchesKey(data, KeyEnter):
		if sl.onSelect != nil && len(sl.filtered) > 0 {
			origIdx := sl.filtered[sl.selectedIdx]
			sl.onSelect(sl.items[origIdx], origIdx)
		}
		return true

	case MatchesKey(data, KeyHome):
		if sl.selectedIdx != 0 {
			sl.selectedIdx = 0
			sl.notifyChange()
			sl.MarkDirty()
		}
		return true

	case MatchesKey(data, KeyEnd):
		last := len(sl.filtered) - 1
		if last >= 0 && sl.selectedIdx != last {
			sl.selectedIdx = last
			sl.notifyChange()
			sl.MarkDirty()
		}
		return true

	case MatchesKey(data, KeyPageUp):
		step := sl.maxVisible
		if step <= 0 {
			step = 10
		}
		sl.selectedIdx -= step
		if sl.selectedIdx < 0 {
			sl.selectedIdx = 0
		}
		sl.notifyChange()
		sl.MarkDirty()
		return true

	case MatchesKey(data, KeyPageDown):
		step := sl.maxVisible
		if step <= 0 {
			step = 10
		}
		sl.selectedIdx += step
		if sl.selectedIdx >= len(sl.filtered) {
			sl.selectedIdx = len(sl.filtered) - 1
		}
		if sl.selectedIdx < 0 {
			sl.selectedIdx = 0
		}
		sl.notifyChange()
		sl.MarkDirty()
		return true

	default:
		// j/k vim-style navigation.
		r, ok := IsRuneInput(data)
		if ok {
			switch r {
			case 'j':
				sl.moveDown()
				return true
			case 'k':
				sl.moveUp()
				return true
			case '/':
				if sl.showFilter {
					sl.filterMode = true
					sl.MarkDirty()
					return true
				}
			}
		}
	}

	return false
}

// handleFilterInput handles input when the filter is active.
func (sl *SelectList) handleFilterInput(data []byte) bool {
	switch {
	case MatchesKey(data, KeyEscape):
		sl.filterMode = false
		sl.MarkDirty()
		return true

	case MatchesKey(data, KeyEnter):
		sl.filterMode = false
		if sl.onSelect != nil && len(sl.filtered) > 0 {
			origIdx := sl.filtered[sl.selectedIdx]
			sl.onSelect(sl.items[origIdx], origIdx)
		}
		return true

	case MatchesKey(data, KeyBackspace):
		if len(sl.filterText) > 0 {
			runes := []rune(sl.filterText)
			sl.filterText = string(runes[:len(runes)-1])
			sl.rebuildFiltered()
			sl.MarkDirty()
		}
		return true

	case MatchesKey(data, KeyUp):
		sl.moveUp()
		return true

	case MatchesKey(data, KeyDown):
		sl.moveDown()
		return true

	default:
		r, ok := IsRuneInput(data)
		if ok {
			sl.filterText += string(r)
			sl.rebuildFiltered()
			sl.MarkDirty()
			return true
		}
	}

	return false
}

// moveUp moves the selection up by one.
func (sl *SelectList) moveUp() {
	if sl.selectedIdx > 0 {
		sl.selectedIdx--
		sl.notifyChange()
		sl.MarkDirty()
	}
}

// moveDown moves the selection down by one.
func (sl *SelectList) moveDown() {
	if sl.selectedIdx < len(sl.filtered)-1 {
		sl.selectedIdx++
		sl.notifyChange()
		sl.MarkDirty()
	}
}

// notifyChange fires the onChange callback with the current selection.
func (sl *SelectList) notifyChange() {
	if sl.onChange != nil && len(sl.filtered) > 0 {
		origIdx := sl.filtered[sl.selectedIdx]
		sl.onChange(sl.items[origIdx], origIdx)
	}
}

// itoa converts an integer to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if neg {
		digits = append(digits, '-')
	}
	// Reverse.
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
