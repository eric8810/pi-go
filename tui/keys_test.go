package tui

import (
	"testing"
)

func TestMatchesKey(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		key  string
		want bool
	}{
		// Enter
		{"enter with CR", []byte{'\r'}, KeyEnter, true},
		{"enter with LF", []byte{'\n'}, KeyEnter, true},
		{"not enter", []byte{'a'}, KeyEnter, false},

		// Escape
		{"escape", []byte{0x1b}, KeyEscape, true},
		{"not escape", []byte{'a'}, KeyEscape, false},
		{"escape sequence is not bare escape", []byte{0x1b, '[', 'A'}, KeyEscape, false},

		// Tab
		{"tab", []byte{'\t'}, KeyTab, true},
		{"not tab", []byte{'a'}, KeyTab, false},

		// Backspace
		{"backspace 0x7f", []byte{0x7f}, KeyBackspace, true},
		{"backspace 0x08", []byte{0x08}, KeyBackspace, true},
		{"not backspace", []byte{'a'}, KeyBackspace, false},

		// Space
		{"space", []byte{' '}, KeySpace, true},
		{"not space", []byte{'a'}, KeySpace, false},

		// Arrow keys
		{"up arrow", []byte{0x1b, '[', 'A'}, KeyUp, true},
		{"down arrow", []byte{0x1b, '[', 'B'}, KeyDown, true},
		{"right arrow", []byte{0x1b, '[', 'C'}, KeyRight, true},
		{"left arrow", []byte{0x1b, '[', 'D'}, KeyLeft, true},
		{"wrong arrow", []byte{0x1b, '[', 'A'}, KeyDown, false},

		// Home/End
		{"home", []byte{0x1b, '[', 'H'}, KeyHome, true},
		{"end", []byte{0x1b, '[', 'F'}, KeyEnd, true},
		{"home alt", []byte("\x1b[1~"), KeyHome, true},
		{"end alt", []byte("\x1b[4~"), KeyEnd, true},

		// Page Up/Down
		{"page up", []byte("\x1b[5~"), KeyPageUp, true},
		{"page down", []byte("\x1b[6~"), KeyPageDown, true},

		// Delete/Insert
		{"delete", []byte("\x1b[3~"), KeyDelete, true},
		{"insert", []byte("\x1b[2~"), KeyInsert, true},

		// Function keys
		{"F1", []byte("\x1bOP"), KeyF1, true},
		{"F2", []byte("\x1bOQ"), KeyF2, true},
		{"F3", []byte("\x1bOR"), KeyF3, true},
		{"F4", []byte("\x1bOS"), KeyF4, true},
		{"F5", []byte("\x1b[15~"), KeyF5, true},

		// Ctrl keys
		{"ctrl+a", []byte{0x01}, KeyCtrlA, true},
		{"ctrl+b", []byte{0x02}, KeyCtrlB, true},
		{"ctrl+c", []byte{0x03}, KeyCtrlC, true},
		{"ctrl+d", []byte{0x04}, KeyCtrlD, true},
		{"ctrl+e", []byte{0x05}, KeyCtrlE, true},
		{"ctrl+f", []byte{0x06}, KeyCtrlF, true},
		{"ctrl+g", []byte{0x07}, KeyCtrlG, true},
		{"ctrl+k", []byte{0x0b}, KeyCtrlK, true},
		{"ctrl+l", []byte{0x0c}, KeyCtrlL, true},
		{"ctrl+n", []byte{0x0e}, KeyCtrlN, true},
		{"ctrl+p", []byte{0x10}, KeyCtrlP, true},
		{"ctrl+u", []byte{0x15}, KeyCtrlU, true},
		{"ctrl+w", []byte{0x17}, KeyCtrlW, true},
		{"ctrl+z", []byte{0x1a}, KeyCtrlZ, true},

		// Ctrl+Left/Right
		{"ctrl+right", []byte("\x1b[1;5C"), KeyCtrlRight, true},
		{"ctrl+left", []byte("\x1b[1;5D"), KeyCtrlLeft, true},
		{"ctrl+right alt", []byte("\x1bOC"), KeyCtrlRight, true},
		{"ctrl+left alt", []byte("\x1bOD"), KeyCtrlLeft, true},

		// Shift+Tab
		{"shift+tab", []byte("\x1b[Z"), KeyShiftTab, true},

		// Shift+Enter
		{"shift+enter CR", []byte("\x1b\r"), KeyShiftEnter, true},
		{"shift+enter LF", []byte("\x1b\n"), KeyShiftEnter, true},

		// Mismatch cases
		{"ctrl+c is not ctrl+d", []byte{0x03}, KeyCtrlD, false},
		{"up is not down", []byte{0x1b, '[', 'A'}, KeyDown, false},

		// Empty data
		{"empty data", []byte{}, KeyEnter, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesKey(tt.data, tt.key)
			if got != tt.want {
				t.Errorf("MatchesKey(%v, %q) = %v, want %v", tt.data, tt.key, got, tt.want)
			}
		})
	}
}

func TestDecodeKey(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"enter CR", []byte{'\r'}, KeyEnter},
		{"enter LF", []byte{'\n'}, KeyEnter},
		{"escape", []byte{0x1b}, KeyEscape},
		{"tab", []byte{'\t'}, KeyTab},
		{"backspace 0x7f", []byte{0x7f}, KeyBackspace},
		{"backspace 0x08", []byte{0x08}, KeyBackspace},
		{"space", []byte{' '}, KeySpace},
		{"up arrow", []byte{0x1b, '[', 'A'}, KeyUp},
		{"down arrow", []byte{0x1b, '[', 'B'}, KeyDown},
		{"right arrow", []byte{0x1b, '[', 'C'}, KeyRight},
		{"left arrow", []byte{0x1b, '[', 'D'}, KeyLeft},
		{"home", []byte{0x1b, '[', 'H'}, KeyHome},
		{"end", []byte{0x1b, '[', 'F'}, KeyEnd},
		{"ctrl+a", []byte{0x01}, KeyCtrlA},
		{"ctrl+c", []byte{0x03}, KeyCtrlC},
		{"ctrl+d", []byte{0x04}, KeyCtrlD},
		{"ctrl+z", []byte{0x1a}, KeyCtrlZ},
		{"F1", []byte("\x1bOP"), KeyF1},
		{"F5", []byte("\x1b[15~"), KeyF5},
		{"shift+tab", []byte("\x1b[Z"), KeyShiftTab},
		{"shift+enter", []byte("\x1b\r"), KeyShiftEnter},
		{"unknown printable", []byte{'a'}, ""},
		{"unknown escape sequence", []byte{0x1b, '[', '9', '9', 'z'}, ""},
		{"empty", []byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecodeKey(tt.data)
			if got != tt.want {
				t.Errorf("DecodeKey(%v) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

func TestIsRuneInput(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantRune rune
		wantOk   bool
	}{
		{
			name:     "printable ASCII letter",
			data:     []byte{'a'},
			wantRune: 'a',
			wantOk:   true,
		},
		{
			name:     "printable ASCII digit",
			data:     []byte{'5'},
			wantRune: '5',
			wantOk:   true,
		},
		{
			name:     "printable ASCII symbol",
			data:     []byte{'@'},
			wantRune: '@',
			wantOk:   true,
		},
		{
			name:     "space is printable",
			data:     []byte{' '},
			wantRune: ' ',
			wantOk:   true,
		},
		{
			name:     "multi-byte UTF8 two bytes",
			data:     []byte{0xc3, 0xa9}, // e-acute
			wantRune: '\u00e9',
			wantOk:   true,
		},
		{
			name:     "multi-byte UTF8 three bytes CJK",
			data:     []byte{0xe4, 0xb8, 0xad}, // U+4E2D
			wantRune: '\u4e2d',
			wantOk:   true,
		},
		{
			name:     "control character CR",
			data:     []byte{'\r'},
			wantRune: 0,
			wantOk:   false,
		},
		{
			name:     "control character LF",
			data:     []byte{'\n'},
			wantRune: 0,
			wantOk:   false,
		},
		{
			name:     "control character ESC",
			data:     []byte{0x1b},
			wantRune: 0,
			wantOk:   false,
		},
		{
			name:     "control character null",
			data:     []byte{0x00},
			wantRune: 0,
			wantOk:   false,
		},
		{
			name:     "DEL is not rune input",
			data:     []byte{0x7f},
			wantRune: 0,
			wantOk:   false,
		},
		{
			name:     "escape sequence is not rune",
			data:     []byte{0x1b, '[', 'A'},
			wantRune: 0,
			wantOk:   false,
		},
		{
			name:     "empty input",
			data:     []byte{},
			wantRune: 0,
			wantOk:   false,
		},
		{
			name:     "tab is not rune input",
			data:     []byte{'\t'},
			wantRune: 0,
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRune, gotOk := IsRuneInput(tt.data)
			if gotOk != tt.wantOk {
				t.Errorf("IsRuneInput(%v) ok = %v, want %v", tt.data, gotOk, tt.wantOk)
			}
			if gotRune != tt.wantRune {
				t.Errorf("IsRuneInput(%v) rune = %q, want %q", tt.data, gotRune, tt.wantRune)
			}
		})
	}
}
