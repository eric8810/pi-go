// Package tui provides terminal user interface components for building
// interactive CLI applications. It includes ANSI escape code utilities,
// text rendering, input handling, and a differential rendering engine.
package tui

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ANSI escape code constants for terminal styling and cursor control.
const (
	// Reset clears all text attributes.
	Reset = "\033[0m"

	// Text style attributes.
	BoldOn          = "\033[1m"
	BoldOff         = "\033[22m"
	DimOn           = "\033[2m"
	DimOff          = "\033[22m"
	ItalicOn        = "\033[3m"
	ItalicOff       = "\033[23m"
	UnderlineOn     = "\033[4m"
	UnderlineOff    = "\033[24m"
	StrikethroughOn = "\033[9m"
	StrikethroughOff = "\033[29m"
	InverseOn       = "\033[7m"
	InverseOff      = "\033[27m"

	// Standard foreground colors.
	FgBlack   = "\033[30m"
	FgRed     = "\033[31m"
	FgGreen   = "\033[32m"
	FgYellow  = "\033[33m"
	FgBlue    = "\033[34m"
	FgMagenta = "\033[35m"
	FgCyan    = "\033[36m"
	FgWhite   = "\033[37m"
	FgDefault = "\033[39m"

	// Bright foreground colors.
	FgBrightBlack   = "\033[90m"
	FgBrightRed     = "\033[91m"
	FgBrightGreen   = "\033[92m"
	FgBrightYellow  = "\033[93m"
	FgBrightBlue    = "\033[94m"
	FgBrightMagenta = "\033[95m"
	FgBrightCyan    = "\033[96m"
	FgBrightWhite   = "\033[97m"

	// Standard background colors.
	BgBlack   = "\033[40m"
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgWhite   = "\033[47m"
	BgDefault = "\033[49m"

	// Bright background colors.
	BgBrightBlack   = "\033[100m"
	BgBrightRed     = "\033[101m"
	BgBrightGreen   = "\033[102m"
	BgBrightYellow  = "\033[103m"
	BgBrightBlue    = "\033[104m"
	BgBrightMagenta = "\033[105m"
	BgBrightCyan    = "\033[106m"
	BgBrightWhite   = "\033[107m"

	// Cursor movement.
	CursorUp       = "\033[A"
	CursorDown     = "\033[B"
	CursorForward  = "\033[C"
	CursorBackward = "\033[D"
	CursorHome     = "\033[H"

	// Screen clearing.
	ClearScreen     = "\033[2J"
	ClearLine       = "\033[2K"
	ClearToEndLine  = "\033[0K"
	ClearFromCursor = "\033[0J"

	// Cursor visibility.
	CursorHide = "\033[?25l"
	CursorShow = "\033[?25h"

	// Synchronized output (atomic updates).
	SyncStart = "\033[?2026h"
	SyncEnd   = "\033[?2026l"
)

// ansiPattern matches all ANSI escape sequences including CSI, OSC, and simple
// two-byte sequences.
var ansiPattern = regexp.MustCompile(`\x1b(?:\[[0-9;]*[a-zA-Z]|\][^\x07]*\x07|[a-zA-Z])`)

// StripAnsi removes all ANSI escape sequences from the string.
func StripAnsi(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

// isWide returns true if the rune is a full-width or wide character according
// to Unicode East Asian Width properties. This covers CJK ideographs,
// fullwidth forms, and other characters that occupy two terminal columns.
func isWide(r rune) bool {
	// CJK Unified Ideographs
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	// CJK Unified Ideographs Extension A
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	// CJK Unified Ideographs Extension B
	if r >= 0x20000 && r <= 0x2A6DF {
		return true
	}
	// CJK Compatibility Ideographs
	if r >= 0xF900 && r <= 0xFAFF {
		return true
	}
	// Hangul Syllables
	if r >= 0xAC00 && r <= 0xD7AF {
		return true
	}
	// Fullwidth Forms
	if r >= 0xFF01 && r <= 0xFF60 {
		return true
	}
	if r >= 0xFFE0 && r <= 0xFFE6 {
		return true
	}
	// CJK Radicals Supplement, Kangxi Radicals
	if r >= 0x2E80 && r <= 0x2FDF {
		return true
	}
	// CJK Symbols and Punctuation, Hiragana, Katakana
	if r >= 0x3000 && r <= 0x303F {
		return true
	}
	if r >= 0x3040 && r <= 0x309F {
		return true
	}
	if r >= 0x30A0 && r <= 0x30FF {
		return true
	}
	// Katakana Phonetic Extensions
	if r >= 0x31F0 && r <= 0x31FF {
		return true
	}
	// Enclosed CJK Letters and Months
	if r >= 0x3200 && r <= 0x32FF {
		return true
	}
	// CJK Compatibility
	if r >= 0x3300 && r <= 0x33FF {
		return true
	}
	// Halfwidth/Fullwidth (additional)
	if r >= 0xFE30 && r <= 0xFE4F {
		return true
	}
	return false
}

// runeWidth returns the number of terminal columns a rune occupies.
func runeWidth(r rune) int {
	if r == 0 {
		return 0
	}
	if unicode.IsControl(r) {
		return 0
	}
	if isWide(r) {
		return 2
	}
	return 1
}

// VisibleWidth returns the display width of a string, ignoring ANSI escape
// sequences and correctly handling double-width CJK characters.
func VisibleWidth(s string) int {
	stripped := StripAnsi(s)
	w := 0
	for _, r := range stripped {
		w += runeWidth(r)
	}
	return w
}

// TruncateToWidth truncates a string to fit within the specified display width,
// preserving ANSI escape codes. If the string is truncated, an ellipsis character
// is appended (if there is room). ANSI codes that were opened before truncation
// are closed with a reset.
func TruncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if VisibleWidth(s) <= width {
		return s
	}

	var buf strings.Builder
	visW := 0
	hasAnsi := false
	i := 0
	bytes := []byte(s)

	for i < len(bytes) {
		// Check for ANSI escape sequence.
		if bytes[i] == 0x1b && i+1 < len(bytes) {
			loc := ansiPattern.FindIndex(bytes[i:])
			if loc != nil && loc[0] == 0 {
				buf.Write(bytes[i : i+loc[1]])
				i += loc[1]
				hasAnsi = true
				continue
			}
		}

		r, size := utf8.DecodeRune(bytes[i:])
		rw := runeWidth(r)

		// Reserve 1 column for ellipsis if we know we'll truncate.
		if visW+rw > width-1 {
			buf.WriteRune('…')
			if hasAnsi {
				buf.WriteString(Reset)
			}
			return buf.String()
		}

		buf.WriteRune(r)
		visW += rw
		i += size
	}

	if hasAnsi {
		buf.WriteString(Reset)
	}
	return buf.String()
}

// WrapText wraps text to the given width, preserving ANSI escape codes and
// respecting word boundaries where possible. Each element in the returned
// slice represents one display line.
func WrapText(s string, width int) []string {
	if width <= 0 {
		return nil
	}

	// Split on explicit newlines first.
	paragraphs := strings.Split(s, "\n")
	var result []string

	for _, para := range paragraphs {
		if VisibleWidth(para) <= width {
			result = append(result, para)
			continue
		}

		lines := wrapParagraph(para, width)
		result = append(result, lines...)
	}

	return result
}

// wrapParagraph wraps a single paragraph (no embedded newlines) to width.
func wrapParagraph(s string, width int) []string {
	if width <= 0 {
		return nil
	}

	var lines []string
	var lineBuf strings.Builder
	var ansiState strings.Builder
	lineW := 0

	words := splitWordsPreservingAnsi(s)

	for wi, word := range words {
		wordW := VisibleWidth(word)

		// If single word is wider than width, break it character by character.
		if wordW > width {
			if lineW > 0 {
				lines = append(lines, lineBuf.String())
				lineBuf.Reset()
				lineBuf.WriteString(ansiState.String())
				lineW = 0
			}
			charLines := breakLongWord(word, width)
			for ci, cl := range charLines {
				if ci < len(charLines)-1 {
					lines = append(lines, cl)
				} else {
					lineBuf.WriteString(cl)
					lineW = VisibleWidth(cl)
				}
			}
			continue
		}

		// Check if word fits on current line.
		neededSpace := 0
		if lineW > 0 {
			neededSpace = 1
		}

		if lineW+neededSpace+wordW > width {
			// Wrap to next line.
			lines = append(lines, lineBuf.String())
			lineBuf.Reset()
			lineBuf.WriteString(ansiState.String())
			lineBuf.WriteString(word)
			lineW = wordW
		} else {
			if lineW > 0 && wi > 0 {
				lineBuf.WriteByte(' ')
				lineW++
			}
			lineBuf.WriteString(word)
			lineW += wordW
		}

		// Track active ANSI state.
		trackAnsiState(word, &ansiState)
	}

	if lineBuf.Len() > 0 {
		lines = append(lines, lineBuf.String())
	}

	if len(lines) == 0 {
		lines = append(lines, "")
	}

	return lines
}

// splitWordsPreservingAnsi splits a string into words while keeping ANSI
// escape codes attached to the words they precede.
func splitWordsPreservingAnsi(s string) []string {
	var words []string
	var current strings.Builder
	i := 0
	bytes := []byte(s)

	for i < len(bytes) {
		// ANSI escape sequence: attach to current word.
		if bytes[i] == 0x1b {
			loc := ansiPattern.FindIndex(bytes[i:])
			if loc != nil && loc[0] == 0 {
				current.Write(bytes[i : i+loc[1]])
				i += loc[1]
				continue
			}
		}

		if bytes[i] == ' ' || bytes[i] == '\t' {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			i++
			continue
		}

		current.WriteByte(bytes[i])
		i++
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// breakLongWord breaks a word that is wider than width into multiple lines.
func breakLongWord(word string, width int) []string {
	var lines []string
	var lineBuf strings.Builder
	lineW := 0
	bytes := []byte(word)
	i := 0

	for i < len(bytes) {
		if bytes[i] == 0x1b {
			loc := ansiPattern.FindIndex(bytes[i:])
			if loc != nil && loc[0] == 0 {
				lineBuf.Write(bytes[i : i+loc[1]])
				i += loc[1]
				continue
			}
		}

		r, size := utf8.DecodeRune(bytes[i:])
		rw := runeWidth(r)

		if lineW+rw > width && lineW > 0 {
			lines = append(lines, lineBuf.String())
			lineBuf.Reset()
			lineW = 0
		}

		lineBuf.WriteRune(r)
		lineW += rw
		i += size
	}

	if lineBuf.Len() > 0 {
		lines = append(lines, lineBuf.String())
	}

	return lines
}

// trackAnsiState updates the running ANSI state by examining sequences in s.
// If a Reset is encountered, the state is cleared.
func trackAnsiState(s string, state *strings.Builder) {
	matches := ansiPattern.FindAllString(s, -1)
	for _, m := range matches {
		if m == Reset {
			state.Reset()
		} else {
			state.WriteString(m)
		}
	}
}

// Bold wraps s in bold ANSI escape codes.
func Bold(s string) string {
	return BoldOn + s + BoldOff
}

// Dim wraps s in dim ANSI escape codes.
func Dim(s string) string {
	return DimOn + s + DimOff
}

// Italic wraps s in italic ANSI escape codes.
func Italic(s string) string {
	return ItalicOn + s + ItalicOff
}

// Underline wraps s in underline ANSI escape codes.
func Underline(s string) string {
	return UnderlineOn + s + UnderlineOff
}

// Strikethrough wraps s in strikethrough ANSI escape codes.
func Strikethrough(s string) string {
	return StrikethroughOn + s + StrikethroughOff
}

// Inverse wraps s in inverse/reverse video ANSI escape codes.
func Inverse(s string) string {
	return InverseOn + s + InverseOff
}

// FgRGB wraps s with a 24-bit true color foreground.
func FgRGB(r, g, b int, s string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s%s", r, g, b, s, Reset)
}

// BgRGB wraps s with a 24-bit true color background.
func BgRGB(r, g, b int, s string) string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm%s%s", r, g, b, s, Reset)
}

// FgColor wraps s with a 256-color foreground. The color parameter is a value
// from 0 to 255 in the terminal's 256-color palette.
func FgColor(color int, s string) string {
	return fmt.Sprintf("\033[38;5;%dm%s%s", color, s, Reset)
}

// BgColor wraps s with a 256-color background. The color parameter is a value
// from 0 to 255 in the terminal's 256-color palette.
func BgColor(color int, s string) string {
	return fmt.Sprintf("\033[48;5;%dm%s%s", color, s, Reset)
}

// CursorTo moves the cursor to the specified row and column (1-based).
func CursorTo(row, col int) string {
	return fmt.Sprintf("\033[%d;%dH", row, col)
}

// CursorMoveUp moves the cursor up by n rows.
func CursorMoveUp(n int) string {
	return fmt.Sprintf("\033[%dA", n)
}

// CursorMoveDown moves the cursor down by n rows.
func CursorMoveDown(n int) string {
	return fmt.Sprintf("\033[%dB", n)
}

// CursorMoveRight moves the cursor right by n columns.
func CursorMoveRight(n int) string {
	return fmt.Sprintf("\033[%dC", n)
}

// CursorMoveLeft moves the cursor left by n columns.
func CursorMoveLeft(n int) string {
	return fmt.Sprintf("\033[%dD", n)
}

// PadRight pads a string with spaces to the given visible width. If the
// string is already wider, it is returned unchanged.
func PadRight(s string, width int) string {
	vw := VisibleWidth(s)
	if vw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vw)
}

// PadCenter centers a string within the given visible width by adding spaces
// on both sides. If the string is already wider, it is returned unchanged.
func PadCenter(s string, width int) string {
	vw := VisibleWidth(s)
	if vw >= width {
		return s
	}
	left := (width - vw) / 2
	right := width - vw - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}
