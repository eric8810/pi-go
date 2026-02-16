package tui

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Input is a single-line text input component that supports cursor movement,
// text editing, horizontal scrolling, and placeholder text.
type Input struct {
	BaseComponent
	focused     bool
	text        []rune
	cursor      int // Cursor position in runes.
	scrollOff   int // Horizontal scroll offset in visible columns.
	placeholder string
	onChange    func(string)
	onSubmit   func(string)
}

// NewInput creates a new single-line text input.
func NewInput() *Input {
	return &Input{
		BaseComponent: BaseComponent{dirty: true},
	}
}

// SetPlaceholder sets the placeholder text shown when the input is empty.
func (inp *Input) SetPlaceholder(s string) {
	inp.placeholder = s
	inp.MarkDirty()
}

// SetOnChange sets a callback invoked whenever the input text changes.
func (inp *Input) SetOnChange(fn func(string)) {
	inp.onChange = fn
}

// SetOnSubmit sets a callback invoked when the user presses Enter.
func (inp *Input) SetOnSubmit(fn func(string)) {
	inp.onSubmit = fn
}

// SetText sets the input text and moves the cursor to the end.
func (inp *Input) SetText(s string) {
	inp.text = []rune(s)
	inp.cursor = len(inp.text)
	inp.MarkDirty()
}

// Text returns the current input text.
func (inp *Input) Text() string {
	return string(inp.text)
}

// SetFocused sets the focus state of this input.
func (inp *Input) SetFocused(focused bool) {
	inp.focused = focused
	inp.MarkDirty()
}

// IsFocused returns whether this input currently has focus.
func (inp *Input) IsFocused() bool {
	return inp.focused
}

// cursorVisiblePos returns the visible column position of the cursor.
func (inp *Input) cursorVisiblePos() int {
	pos := 0
	for i := 0; i < inp.cursor && i < len(inp.text); i++ {
		pos += runeWidth(inp.text[i])
	}
	return pos
}

// Render produces a single line representing the input field.
func (inp *Input) Render(width int) []string {
	if width <= 0 {
		return []string{""}
	}

	if len(inp.text) == 0 && !inp.focused {
		// Show placeholder.
		ph := inp.placeholder
		if VisibleWidth(ph) > width {
			ph = TruncateToWidth(ph, width)
		}
		return []string{DimOn + ph + DimOff}
	}

	if len(inp.text) == 0 && inp.focused {
		// Empty with cursor.
		if inp.placeholder != "" {
			ph := inp.placeholder
			if VisibleWidth(ph) > width-1 {
				ph = TruncateToWidth(ph, width-1)
			}
			return []string{InverseOn + " " + InverseOff + DimOn + ph + DimOff}
		}
		return []string{InverseOn + " " + InverseOff}
	}

	// Ensure cursor is within the visible window.
	cursorCol := inp.cursorVisiblePos()

	// Adjust scroll offset so cursor is visible.
	if cursorCol < inp.scrollOff {
		inp.scrollOff = cursorCol
	}
	if cursorCol >= inp.scrollOff+width {
		inp.scrollOff = cursorCol - width + 1
	}

	// Build the visible portion of the text.
	var buf strings.Builder
	visCol := 0
	for i, r := range inp.text {
		rw := runeWidth(r)
		endCol := visCol + rw

		if endCol <= inp.scrollOff {
			visCol = endCol
			continue
		}

		if visCol >= inp.scrollOff+width {
			break
		}

		if inp.focused && i == inp.cursor {
			buf.WriteString(InverseOn)
			buf.WriteRune(r)
			buf.WriteString(InverseOff)
		} else {
			buf.WriteRune(r)
		}
		visCol = endCol
	}

	// If cursor is at the end of text, show cursor block.
	if inp.focused && inp.cursor == len(inp.text) {
		if visCol >= inp.scrollOff && visCol < inp.scrollOff+width {
			buf.WriteString(InverseOn + " " + InverseOff)
		}
	}

	return []string{buf.String()}
}

// HandleInput processes keyboard input for the input field.
func (inp *Input) HandleInput(data []byte) bool {
	switch {
	case MatchesKey(data, KeyEnter):
		if inp.onSubmit != nil {
			inp.onSubmit(string(inp.text))
		}
		return true

	case MatchesKey(data, KeyBackspace):
		if inp.cursor > 0 {
			inp.text = append(inp.text[:inp.cursor-1], inp.text[inp.cursor:]...)
			inp.cursor--
			inp.changed()
		}
		return true

	case MatchesKey(data, KeyDelete):
		if inp.cursor < len(inp.text) {
			inp.text = append(inp.text[:inp.cursor], inp.text[inp.cursor+1:]...)
			inp.changed()
		}
		return true

	case MatchesKey(data, KeyLeft):
		if inp.cursor > 0 {
			inp.cursor--
			inp.MarkDirty()
		}
		return true

	case MatchesKey(data, KeyRight):
		if inp.cursor < len(inp.text) {
			inp.cursor++
			inp.MarkDirty()
		}
		return true

	case MatchesKey(data, KeyHome), MatchesKey(data, KeyCtrlA):
		if inp.cursor != 0 {
			inp.cursor = 0
			inp.MarkDirty()
		}
		return true

	case MatchesKey(data, KeyEnd), MatchesKey(data, KeyCtrlE):
		if inp.cursor != len(inp.text) {
			inp.cursor = len(inp.text)
			inp.MarkDirty()
		}
		return true

	case MatchesKey(data, KeyCtrlLeft):
		inp.cursor = inp.prevWordBoundary()
		inp.MarkDirty()
		return true

	case MatchesKey(data, KeyCtrlRight):
		inp.cursor = inp.nextWordBoundary()
		inp.MarkDirty()
		return true

	case MatchesKey(data, KeyCtrlK):
		// Kill from cursor to end of line.
		if inp.cursor < len(inp.text) {
			inp.text = inp.text[:inp.cursor]
			inp.changed()
		}
		return true

	case MatchesKey(data, KeyCtrlU):
		// Clear line (delete everything before cursor).
		if inp.cursor > 0 {
			inp.text = inp.text[inp.cursor:]
			inp.cursor = 0
			inp.changed()
		}
		return true

	case MatchesKey(data, KeyCtrlW):
		// Delete word before cursor.
		if inp.cursor > 0 {
			newPos := inp.prevWordBoundary()
			inp.text = append(inp.text[:newPos], inp.text[inp.cursor:]...)
			inp.cursor = newPos
			inp.changed()
		}
		return true

	default:
		// Try to insert printable rune.
		r, ok := IsRuneInput(data)
		if ok {
			inp.insertRune(r)
			return true
		}

		// Handle multi-byte UTF-8 that might be paste.
		if len(data) > 1 && data[0] != 0x1b {
			s := string(data)
			runes := []rune(s)
			for _, r := range runes {
				if r >= 0x20 && r != utf8.RuneError {
					inp.insertRune(r)
				}
			}
			return true
		}
	}

	return false
}

// insertRune inserts a rune at the cursor position.
func (inp *Input) insertRune(r rune) {
	newText := make([]rune, len(inp.text)+1)
	copy(newText, inp.text[:inp.cursor])
	newText[inp.cursor] = r
	copy(newText[inp.cursor+1:], inp.text[inp.cursor:])
	inp.text = newText
	inp.cursor++
	inp.changed()
}

// changed triggers the onChange callback and marks dirty.
func (inp *Input) changed() {
	inp.MarkDirty()
	if inp.onChange != nil {
		inp.onChange(string(inp.text))
	}
}

// prevWordBoundary finds the start of the previous word.
func (inp *Input) prevWordBoundary() int {
	if inp.cursor <= 0 {
		return 0
	}
	pos := inp.cursor - 1
	// Skip trailing spaces.
	for pos > 0 && unicode.IsSpace(inp.text[pos]) {
		pos--
	}
	// Skip word characters.
	for pos > 0 && !unicode.IsSpace(inp.text[pos-1]) {
		pos--
	}
	return pos
}

// nextWordBoundary finds the start of the next word.
func (inp *Input) nextWordBoundary() int {
	if inp.cursor >= len(inp.text) {
		return len(inp.text)
	}
	pos := inp.cursor
	// Skip current word.
	for pos < len(inp.text) && !unicode.IsSpace(inp.text[pos]) {
		pos++
	}
	// Skip spaces.
	for pos < len(inp.text) && unicode.IsSpace(inp.text[pos]) {
		pos++
	}
	return pos
}
