package tui

import (
	"strings"
	"unicode/utf8"
)

// Editor is a multi-line text editor component with vertical scrolling,
// cursor navigation, and configurable submit behavior.
type Editor struct {
	BaseComponent
	focused    bool
	lines      [][]rune // Each line as a slice of runes.
	cursorRow  int
	cursorCol  int
	scrollTop  int    // First visible line index.
	maxVisible int    // Maximum visible lines (0 = unlimited).
	submitKey  string // Key that triggers submit (default: KeyEnter without shift).
	onChange   func(string)
	onSubmit   func(string)
	useShiftEnterSubmit bool // If true, Enter inserts newline, Shift+Enter submits.
}

// NewEditor creates a new multi-line text editor.
func NewEditor() *Editor {
	return &Editor{
		BaseComponent: BaseComponent{dirty: true},
		lines:         [][]rune{{}},
		maxVisible:    10,
	}
}

// SetMaxVisible sets the maximum number of visible lines before scrolling.
func (e *Editor) SetMaxVisible(n int) {
	e.maxVisible = n
	e.MarkDirty()
}

// SetOnChange sets a callback invoked whenever the editor content changes.
func (e *Editor) SetOnChange(fn func(string)) {
	e.onChange = fn
}

// SetOnSubmit sets a callback invoked when the submit key is pressed.
func (e *Editor) SetOnSubmit(fn func(string)) {
	e.onSubmit = fn
}

// SetSubmitKey sets the key that triggers submission. Use empty string for
// default behavior (Enter submits unless UseShiftEnterSubmit is set).
func (e *Editor) SetSubmitKey(key string) {
	e.submitKey = key
}

// SetUseShiftEnterSubmit controls whether Enter inserts a newline and
// Shift+Enter submits (true), or Enter submits (false, default).
func (e *Editor) SetUseShiftEnterSubmit(v bool) {
	e.useShiftEnterSubmit = v
}

// SetText replaces the editor content and moves the cursor to the end.
func (e *Editor) SetText(s string) {
	rawLines := strings.Split(s, "\n")
	e.lines = make([][]rune, len(rawLines))
	for i, l := range rawLines {
		e.lines[i] = []rune(l)
	}
	e.cursorRow = len(e.lines) - 1
	e.cursorCol = len(e.lines[e.cursorRow])
	e.MarkDirty()
}

// Text returns the full editor content as a single string.
func (e *Editor) Text() string {
	strs := make([]string, len(e.lines))
	for i, l := range e.lines {
		strs[i] = string(l)
	}
	return strings.Join(strs, "\n")
}

// SetFocused sets the focus state.
func (e *Editor) SetFocused(focused bool) {
	e.focused = focused
	e.MarkDirty()
}

// IsFocused returns whether this editor currently has focus.
func (e *Editor) IsFocused() bool {
	return e.focused
}

// ensureScrollVisible adjusts scrollTop so the cursor row is visible.
func (e *Editor) ensureScrollVisible() {
	if e.maxVisible <= 0 {
		e.scrollTop = 0
		return
	}
	if e.cursorRow < e.scrollTop {
		e.scrollTop = e.cursorRow
	}
	if e.cursorRow >= e.scrollTop+e.maxVisible {
		e.scrollTop = e.cursorRow - e.maxVisible + 1
	}
}

// Render produces the visible lines of the editor.
func (e *Editor) Render(width int) []string {
	if width <= 0 {
		return []string{""}
	}

	e.ensureScrollVisible()

	visibleCount := len(e.lines)
	if e.maxVisible > 0 && visibleCount > e.maxVisible {
		visibleCount = e.maxVisible
	}

	endLine := e.scrollTop + visibleCount
	if endLine > len(e.lines) {
		endLine = len(e.lines)
	}

	result := make([]string, 0, visibleCount)

	for i := e.scrollTop; i < endLine; i++ {
		line := e.renderLine(i, width)
		result = append(result, line)
	}

	// Pad to maxVisible if needed.
	for len(result) < visibleCount {
		result = append(result, "")
	}

	return result
}

// renderLine renders a single editor line with cursor highlight if focused.
func (e *Editor) renderLine(lineIdx int, width int) string {
	lineRunes := e.lines[lineIdx]

	if !e.focused || lineIdx != e.cursorRow {
		s := string(lineRunes)
		if VisibleWidth(s) > width {
			return TruncateToWidth(s, width)
		}
		return s
	}

	// This is the cursor line. Show cursor.
	var buf strings.Builder
	col := e.cursorCol
	if col > len(lineRunes) {
		col = len(lineRunes)
	}

	for i, r := range lineRunes {
		if i == col {
			buf.WriteString(InverseOn)
			buf.WriteRune(r)
			buf.WriteString(InverseOff)
		} else {
			buf.WriteRune(r)
		}
	}

	// Cursor at end of line.
	if col >= len(lineRunes) {
		buf.WriteString(InverseOn + " " + InverseOff)
	}

	result := buf.String()
	if VisibleWidth(result) > width {
		result = TruncateToWidth(result, width)
	}
	return result
}

// HandleInput processes keyboard input for the editor.
func (e *Editor) HandleInput(data []byte) bool {
	// Submit handling.
	if e.submitKey != "" && MatchesKey(data, e.submitKey) {
		if e.onSubmit != nil {
			e.onSubmit(e.Text())
		}
		return true
	}

	if e.useShiftEnterSubmit {
		if MatchesKey(data, KeyShiftEnter) {
			if e.onSubmit != nil {
				e.onSubmit(e.Text())
			}
			return true
		}
		if MatchesKey(data, KeyEnter) {
			e.insertNewline()
			return true
		}
	} else {
		if MatchesKey(data, KeyEnter) {
			if e.onSubmit != nil {
				e.onSubmit(e.Text())
			}
			return true
		}
		if MatchesKey(data, KeyShiftEnter) {
			e.insertNewline()
			return true
		}
	}

	switch {
	case MatchesKey(data, KeyBackspace):
		e.backspace()
		return true

	case MatchesKey(data, KeyDelete):
		e.deleteForward()
		return true

	case MatchesKey(data, KeyLeft):
		e.moveLeft()
		return true

	case MatchesKey(data, KeyRight):
		e.moveRight()
		return true

	case MatchesKey(data, KeyUp):
		e.moveUp()
		return true

	case MatchesKey(data, KeyDown):
		e.moveDown()
		return true

	case MatchesKey(data, KeyHome), MatchesKey(data, KeyCtrlA):
		e.cursorCol = 0
		e.MarkDirty()
		return true

	case MatchesKey(data, KeyEnd), MatchesKey(data, KeyCtrlE):
		e.cursorCol = len(e.lines[e.cursorRow])
		e.MarkDirty()
		return true

	case MatchesKey(data, KeyCtrlD):
		// Delete current line.
		e.deleteLine()
		return true

	case MatchesKey(data, KeyCtrlK):
		// Kill to end of line.
		if e.cursorCol < len(e.lines[e.cursorRow]) {
			e.lines[e.cursorRow] = e.lines[e.cursorRow][:e.cursorCol]
			e.changed()
		}
		return true

	case MatchesKey(data, KeyCtrlU):
		// Clear from start to cursor.
		if e.cursorCol > 0 {
			e.lines[e.cursorRow] = e.lines[e.cursorRow][e.cursorCol:]
			e.cursorCol = 0
			e.changed()
		}
		return true

	default:
		// Handle printable rune input.
		r, ok := IsRuneInput(data)
		if ok {
			e.insertRune(r)
			return true
		}

		// Handle multi-byte paste.
		if len(data) > 1 && data[0] != 0x1b {
			s := string(data)
			// Multi-line paste detection.
			if strings.Contains(s, "\n") || strings.Contains(s, "\r") {
				e.insertText(s)
				return true
			}
			// Single line paste.
			for _, r := range s {
				if r >= 0x20 && r != utf8.RuneError {
					e.insertRune(r)
				}
			}
			return true
		}
	}

	return false
}

// insertNewline inserts a newline at the cursor position, splitting the line.
func (e *Editor) insertNewline() {
	line := e.lines[e.cursorRow]
	col := e.cursorCol
	if col > len(line) {
		col = len(line)
	}

	before := make([]rune, col)
	copy(before, line[:col])
	after := make([]rune, len(line)-col)
	copy(after, line[col:])

	e.lines[e.cursorRow] = before
	newLines := make([][]rune, len(e.lines)+1)
	copy(newLines, e.lines[:e.cursorRow+1])
	newLines[e.cursorRow+1] = after
	copy(newLines[e.cursorRow+2:], e.lines[e.cursorRow+1:])
	e.lines = newLines

	e.cursorRow++
	e.cursorCol = 0
	e.changed()
}

// insertRune inserts a rune at the cursor position.
func (e *Editor) insertRune(r rune) {
	line := e.lines[e.cursorRow]
	col := e.cursorCol
	if col > len(line) {
		col = len(line)
	}

	newLine := make([]rune, len(line)+1)
	copy(newLine, line[:col])
	newLine[col] = r
	copy(newLine[col+1:], line[col:])
	e.lines[e.cursorRow] = newLine
	e.cursorCol = col + 1
	e.changed()
}

// insertText inserts multi-line text at the cursor position.
func (e *Editor) insertText(s string) {
	// Normalize line endings.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	pasteLines := strings.Split(s, "\n")

	if len(pasteLines) == 0 {
		return
	}

	currentLine := e.lines[e.cursorRow]
	col := e.cursorCol
	if col > len(currentLine) {
		col = len(currentLine)
	}

	before := currentLine[:col]
	after := currentLine[col:]

	if len(pasteLines) == 1 {
		// Single line paste.
		newLine := make([]rune, 0, len(before)+len([]rune(pasteLines[0]))+len(after))
		newLine = append(newLine, before...)
		newLine = append(newLine, []rune(pasteLines[0])...)
		newLine = append(newLine, after...)
		e.lines[e.cursorRow] = newLine
		e.cursorCol = len(before) + len([]rune(pasteLines[0]))
	} else {
		// Multi-line paste.
		firstLine := make([]rune, 0, len(before)+len([]rune(pasteLines[0])))
		firstLine = append(firstLine, before...)
		firstLine = append(firstLine, []rune(pasteLines[0])...)

		lastPaste := []rune(pasteLines[len(pasteLines)-1])
		lastLine := make([]rune, 0, len(lastPaste)+len(after))
		lastLine = append(lastLine, lastPaste...)
		lastLine = append(lastLine, after...)

		newLines := make([][]rune, 0, len(e.lines)+len(pasteLines)-1)
		newLines = append(newLines, e.lines[:e.cursorRow]...)
		newLines = append(newLines, firstLine)
		for i := 1; i < len(pasteLines)-1; i++ {
			newLines = append(newLines, []rune(pasteLines[i]))
		}
		newLines = append(newLines, lastLine)
		newLines = append(newLines, e.lines[e.cursorRow+1:]...)

		e.lines = newLines
		e.cursorRow = e.cursorRow + len(pasteLines) - 1
		e.cursorCol = len(lastPaste)
	}

	e.changed()
}

// backspace deletes the character before the cursor, or joins with previous line.
func (e *Editor) backspace() {
	if e.cursorCol > 0 {
		line := e.lines[e.cursorRow]
		col := e.cursorCol
		if col > len(line) {
			col = len(line)
		}
		newLine := make([]rune, len(line)-1)
		copy(newLine, line[:col-1])
		copy(newLine[col-1:], line[col:])
		e.lines[e.cursorRow] = newLine
		e.cursorCol = col - 1
		e.changed()
	} else if e.cursorRow > 0 {
		// Join with previous line.
		prevLine := e.lines[e.cursorRow-1]
		curLine := e.lines[e.cursorRow]
		joinCol := len(prevLine)

		joined := make([]rune, len(prevLine)+len(curLine))
		copy(joined, prevLine)
		copy(joined[len(prevLine):], curLine)

		e.lines[e.cursorRow-1] = joined
		e.lines = append(e.lines[:e.cursorRow], e.lines[e.cursorRow+1:]...)
		e.cursorRow--
		e.cursorCol = joinCol
		e.changed()
	}
}

// deleteForward deletes the character under the cursor, or joins with next line.
func (e *Editor) deleteForward() {
	line := e.lines[e.cursorRow]
	col := e.cursorCol
	if col > len(line) {
		col = len(line)
	}
	if col < len(line) {
		newLine := make([]rune, len(line)-1)
		copy(newLine, line[:col])
		copy(newLine[col:], line[col+1:])
		e.lines[e.cursorRow] = newLine
		e.changed()
	} else if e.cursorRow < len(e.lines)-1 {
		// Join with next line.
		nextLine := e.lines[e.cursorRow+1]
		joined := make([]rune, len(line)+len(nextLine))
		copy(joined, line)
		copy(joined[len(line):], nextLine)

		e.lines[e.cursorRow] = joined
		e.lines = append(e.lines[:e.cursorRow+1], e.lines[e.cursorRow+2:]...)
		e.changed()
	}
}

// deleteLine deletes the current line.
func (e *Editor) deleteLine() {
	if len(e.lines) == 1 {
		e.lines[0] = nil
		e.cursorCol = 0
		e.changed()
		return
	}

	e.lines = append(e.lines[:e.cursorRow], e.lines[e.cursorRow+1:]...)
	if e.cursorRow >= len(e.lines) {
		e.cursorRow = len(e.lines) - 1
	}
	if e.cursorCol > len(e.lines[e.cursorRow]) {
		e.cursorCol = len(e.lines[e.cursorRow])
	}
	e.changed()
}

// moveLeft moves the cursor one position to the left.
func (e *Editor) moveLeft() {
	if e.cursorCol > 0 {
		e.cursorCol--
	} else if e.cursorRow > 0 {
		e.cursorRow--
		e.cursorCol = len(e.lines[e.cursorRow])
	}
	e.MarkDirty()
}

// moveRight moves the cursor one position to the right.
func (e *Editor) moveRight() {
	if e.cursorCol < len(e.lines[e.cursorRow]) {
		e.cursorCol++
	} else if e.cursorRow < len(e.lines)-1 {
		e.cursorRow++
		e.cursorCol = 0
	}
	e.MarkDirty()
}

// moveUp moves the cursor up one line.
func (e *Editor) moveUp() {
	if e.cursorRow > 0 {
		e.cursorRow--
		if e.cursorCol > len(e.lines[e.cursorRow]) {
			e.cursorCol = len(e.lines[e.cursorRow])
		}
		e.MarkDirty()
	}
}

// moveDown moves the cursor down one line.
func (e *Editor) moveDown() {
	if e.cursorRow < len(e.lines)-1 {
		e.cursorRow++
		if e.cursorCol > len(e.lines[e.cursorRow]) {
			e.cursorCol = len(e.lines[e.cursorRow])
		}
		e.MarkDirty()
	}
}

// changed triggers the onChange callback and marks dirty.
func (e *Editor) changed() {
	e.MarkDirty()
	if e.onChange != nil {
		e.onChange(e.Text())
	}
}

// CursorPosition returns the current cursor row and column.
func (e *Editor) CursorPosition() (row, col int) {
	return e.cursorRow, e.cursorCol
}

// LineCount returns the number of lines in the editor.
func (e *Editor) LineCount() int {
	return len(e.lines)
}

