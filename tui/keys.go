package tui

// Key constants for special terminal input sequences.
// These values can be used with MatchesKey to identify specific key presses
// from raw terminal input.
const (
	KeyEnter     = "enter"
	KeyEscape    = "escape"
	KeyTab       = "tab"
	KeyBackspace = "backspace"
	KeyDelete    = "delete"
	KeySpace     = "space"

	KeyUp    = "up"
	KeyDown  = "down"
	KeyLeft  = "left"
	KeyRight = "right"

	KeyHome     = "home"
	KeyEnd      = "end"
	KeyPageUp   = "pageup"
	KeyPageDown = "pagedown"
	KeyInsert   = "insert"

	KeyF1  = "f1"
	KeyF2  = "f2"
	KeyF3  = "f3"
	KeyF4  = "f4"
	KeyF5  = "f5"
	KeyF6  = "f6"
	KeyF7  = "f7"
	KeyF8  = "f8"
	KeyF9  = "f9"
	KeyF10 = "f10"
	KeyF11 = "f11"
	KeyF12 = "f12"

	KeyCtrlA = "ctrl+a"
	KeyCtrlB = "ctrl+b"
	KeyCtrlC = "ctrl+c"
	KeyCtrlD = "ctrl+d"
	KeyCtrlE = "ctrl+e"
	KeyCtrlF = "ctrl+f"
	KeyCtrlG = "ctrl+g"
	KeyCtrlH = "ctrl+h"
	KeyCtrlK = "ctrl+k"
	KeyCtrlL = "ctrl+l"
	KeyCtrlN = "ctrl+n"
	KeyCtrlP = "ctrl+p"
	KeyCtrlR = "ctrl+r"
	KeyCtrlT = "ctrl+t"
	KeyCtrlU = "ctrl+u"
	KeyCtrlW = "ctrl+w"
	KeyCtrlX = "ctrl+x"
	KeyCtrlY = "ctrl+y"
	KeyCtrlZ = "ctrl+z"

	KeyShiftTab   = "shift+tab"
	KeyCtrlLeft   = "ctrl+left"
	KeyCtrlRight  = "ctrl+right"
	KeyShiftEnter = "shift+enter"
)

// escSequences maps escape sequences from the terminal to key names.
var escSequences = map[string]string{
	"\x1b[A":    KeyUp,
	"\x1b[B":    KeyDown,
	"\x1b[C":    KeyRight,
	"\x1b[D":    KeyLeft,
	"\x1b[H":    KeyHome,
	"\x1b[F":    KeyEnd,
	"\x1b[1~":   KeyHome,
	"\x1b[4~":   KeyEnd,
	"\x1b[2~":   KeyInsert,
	"\x1b[3~":   KeyDelete,
	"\x1b[5~":   KeyPageUp,
	"\x1b[6~":   KeyPageDown,
	"\x1bOP":    KeyF1,
	"\x1bOQ":    KeyF2,
	"\x1bOR":    KeyF3,
	"\x1bOS":    KeyF4,
	"\x1b[15~":  KeyF5,
	"\x1b[17~":  KeyF6,
	"\x1b[18~":  KeyF7,
	"\x1b[19~":  KeyF8,
	"\x1b[20~":  KeyF9,
	"\x1b[21~":  KeyF10,
	"\x1b[23~":  KeyF11,
	"\x1b[24~":  KeyF12,
	"\x1b[Z":    KeyShiftTab,
	"\x1b[1;5C": KeyCtrlRight,
	"\x1b[1;5D": KeyCtrlLeft,
	"\x1bOC":    KeyCtrlRight,
	"\x1bOD":    KeyCtrlLeft,
}

// ctrlKeys maps control character byte values (0x01-0x1a) to key names.
var ctrlKeys = map[byte]string{
	0x01: KeyCtrlA,
	0x02: KeyCtrlB,
	0x03: KeyCtrlC,
	0x04: KeyCtrlD,
	0x05: KeyCtrlE,
	0x06: KeyCtrlF,
	0x07: KeyCtrlG,
	// 0x08 is backspace on some terminals
	0x0b: KeyCtrlK,
	0x0c: KeyCtrlL,
	0x0e: KeyCtrlN,
	0x10: KeyCtrlP,
	0x12: KeyCtrlR,
	0x14: KeyCtrlT,
	0x15: KeyCtrlU,
	0x17: KeyCtrlW,
	0x18: KeyCtrlX,
	0x19: KeyCtrlY,
	0x1a: KeyCtrlZ,
}

// MatchesKey checks if the raw terminal input data matches the specified key
// constant. The key parameter should be one of the Key* constants defined in
// this package (e.g., KeyEnter, KeyCtrlC, KeyUp).
func MatchesKey(data []byte, key string) bool {
	if len(data) == 0 {
		return false
	}

	switch key {
	case KeyEnter:
		return len(data) == 1 && (data[0] == '\r' || data[0] == '\n')
	case KeyEscape:
		return len(data) == 1 && data[0] == 0x1b
	case KeyTab:
		return len(data) == 1 && data[0] == '\t'
	case KeyBackspace:
		return len(data) == 1 && (data[0] == 0x7f || data[0] == 0x08)
	case KeySpace:
		return len(data) == 1 && data[0] == ' '
	case KeyShiftEnter:
		// Some terminals send ESC followed by enter for shift+enter.
		return string(data) == "\x1b\r" || string(data) == "\x1b\n" || string(data) == "\x1b[13;2u"
	}

	// Check escape sequences.
	s := string(data)
	for seq, name := range escSequences {
		if name == key && s == seq {
			return true
		}
	}

	// Check control keys.
	if len(data) == 1 {
		if name, ok := ctrlKeys[data[0]]; ok {
			return name == key
		}
		// Also handle ctrl+h as backspace.
		if data[0] == 0x08 && key == KeyCtrlH {
			return true
		}
	}

	return false
}

// DecodeKey attempts to decode raw terminal input into a key name. If the
// input does not match any known key, an empty string is returned.
func DecodeKey(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	if len(data) == 1 {
		switch data[0] {
		case '\r', '\n':
			return KeyEnter
		case 0x1b:
			return KeyEscape
		case '\t':
			return KeyTab
		case 0x7f:
			return KeyBackspace
		case 0x08:
			return KeyBackspace
		case ' ':
			return KeySpace
		}

		if name, ok := ctrlKeys[data[0]]; ok {
			return name
		}

		return ""
	}

	s := string(data)

	// Check shift+enter sequences.
	if s == "\x1b\r" || s == "\x1b\n" || s == "\x1b[13;2u" {
		return KeyShiftEnter
	}

	if name, ok := escSequences[s]; ok {
		return name
	}

	return ""
}

// IsRuneInput returns true if the input represents a printable rune (not a
// control character or escape sequence) and returns that rune.
func IsRuneInput(data []byte) (rune, bool) {
	if len(data) == 0 {
		return 0, false
	}
	// Escape sequences are not rune input.
	if data[0] == 0x1b {
		return 0, false
	}
	// Control characters are not rune input (except tab handled separately).
	if data[0] < 0x20 || data[0] == 0x7f {
		return 0, false
	}
	r, size := decodeRuneFromBytes(data)
	if r == 0 || size == 0 {
		return 0, false
	}
	return r, true
}

// decodeRuneFromBytes decodes the first rune from a byte slice.
func decodeRuneFromBytes(data []byte) (rune, int) {
	if len(data) == 0 {
		return 0, 0
	}
	r, size := rune(data[0]), 1
	if data[0] >= 0x80 {
		var ok bool
		r, size, ok = decodeUTF8(data)
		if !ok {
			return 0, 0
		}
	}
	return r, size
}

// decodeUTF8 decodes a UTF-8 encoded rune from the byte slice.
func decodeUTF8(data []byte) (rune, int, bool) {
	if len(data) == 0 {
		return 0, 0, false
	}
	b := data[0]
	var size int
	var min rune
	switch {
	case b < 0xC0:
		return 0, 0, false
	case b < 0xE0:
		size = 2
		min = 0x80
	case b < 0xF0:
		size = 3
		min = 0x800
	case b < 0xF8:
		size = 4
		min = 0x10000
	default:
		return 0, 0, false
	}
	if len(data) < size {
		return 0, 0, false
	}
	r := rune(b) & (0xFF >> (size + 1))
	for i := 1; i < size; i++ {
		if data[i]&0xC0 != 0x80 {
			return 0, 0, false
		}
		r = r<<6 | rune(data[i])&0x3F
	}
	if r < min {
		return 0, 0, false
	}
	return r, size, true
}
