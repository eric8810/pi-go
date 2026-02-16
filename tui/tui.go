package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// OverlayPosition specifies where an overlay is rendered.
type OverlayPosition int

const (
	// OverlayCenter centers the overlay on screen.
	OverlayCenter OverlayPosition = iota
	// OverlayTop anchors the overlay to the top.
	OverlayTop
	// OverlayBottom anchors the overlay to the bottom.
	OverlayBottom
)

// OverlayOptions configures how an overlay is displayed.
type OverlayOptions struct {
	Position OverlayPosition
	Width    int // 0 means use terminal width.
	Height   int // 0 means auto-size to content.
}

// Overlay holds a component rendered on top of the main content.
type Overlay struct {
	component Component
	options   OverlayOptions
}

// TUI is the terminal user interface renderer. It manages a root component
// tree, handles terminal raw mode, input routing, differential rendering,
// and overlay management.
type TUI struct {
	root      Component
	writer    io.Writer
	width     int
	height    int
	lastLines []string
	overlays  []Overlay
	focus     Focusable
	mu        sync.Mutex

	// Terminal state for raw mode.
	origTermios *syscall.Termios
	rawMode     bool
	inputCh     chan []byte

	// onRender is called after each render (useful for testing).
	onRender func()

	// onInput is called for unhandled input.
	onInput func(data []byte)

	// quit channel for stopping the run loop.
	quit chan struct{}
}

// New creates a new TUI with the given root component.
func New(root Component) *TUI {
	return &TUI{
		root:   root,
		writer: os.Stdout,
		quit:   make(chan struct{}),
	}
}

// SetRoot replaces the root component.
func (t *TUI) SetRoot(root Component) {
	t.mu.Lock()
	t.root = root
	t.lastLines = nil // Force full re-render.
	t.mu.Unlock()
}

// Root returns the current root component.
func (t *TUI) Root() Component {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.root
}

// SetWriter sets the output writer (defaults to os.Stdout).
func (t *TUI) SetWriter(w io.Writer) {
	t.mu.Lock()
	t.writer = w
	t.mu.Unlock()
}

// SetOnRender sets a callback invoked after each render.
func (t *TUI) SetOnRender(fn func()) {
	t.mu.Lock()
	t.onRender = fn
	t.mu.Unlock()
}

// SetOnInput sets a callback for unhandled input.
func (t *TUI) SetOnInput(fn func(data []byte)) {
	t.mu.Lock()
	t.onInput = fn
	t.mu.Unlock()
}

// SetFocus sets which Focusable component receives keyboard input.
func (t *TUI) SetFocus(f Focusable) {
	t.mu.Lock()
	if t.focus != nil {
		t.focus.SetFocused(false)
	}
	t.focus = f
	if f != nil {
		f.SetFocused(true)
	}
	t.mu.Unlock()
}

// Focus returns the currently focused component.
func (t *TUI) Focus() Focusable {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.focus
}

// ShowOverlay adds an overlay on top of the main content. Returns a function
// that removes the overlay when called.
func (t *TUI) ShowOverlay(component Component, opts OverlayOptions) func() {
	t.mu.Lock()
	overlay := Overlay{component: component, options: opts}
	t.overlays = append(t.overlays, overlay)
	idx := len(t.overlays) - 1
	t.mu.Unlock()

	return func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if idx < len(t.overlays) {
			t.overlays = append(t.overlays[:idx], t.overlays[idx+1:]...)
		}
		t.lastLines = nil // Force full re-render.
	}
}

// ClearOverlays removes all overlays.
func (t *TUI) ClearOverlays() {
	t.mu.Lock()
	t.overlays = nil
	t.lastLines = nil
	t.mu.Unlock()
}

// Size returns the current terminal dimensions.
func (t *TUI) Size() (width, height int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.width, t.height
}

// winsize is the struct used by the TIOCGWINSZ ioctl.
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// updateSize reads the current terminal size via ioctl.
func (t *TUI) updateSize() {
	var ws winsize
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		syscall.TIOCGWINSZ,
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 {
		// Fallback defaults.
		t.width = 80
		t.height = 24
		return
	}
	t.width = int(ws.Col)
	t.height = int(ws.Row)
}

// ioctlGetTermios retrieves the terminal attributes.
func ioctlGetTermios(fd uintptr) (*syscall.Termios, error) {
	var t syscall.Termios
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&t)),
	)
	if errno != 0 {
		return nil, errno
	}
	return &t, nil
}

// ioctlSetTermios sets the terminal attributes.
func ioctlSetTermios(fd uintptr, termios *syscall.Termios) error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(termios)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// enterRawMode puts the terminal into raw mode for character-by-character input.
func (t *TUI) enterRawMode() error {
	fd := os.Stdin.Fd()
	termios, err := ioctlGetTermios(fd)
	if err != nil {
		return fmt.Errorf("get termios: %w", err)
	}

	t.origTermios = termios

	raw := *termios
	// Input flags: disable break, CR-to-NL, parity, strip, flow control.
	raw.Iflag &^= syscall.BRKINT | syscall.ICRNL | syscall.INPCK | syscall.ISTRIP | syscall.IXON
	// Output flags: disable post-processing.
	raw.Oflag &^= syscall.OPOST
	// Control flags: set 8-bit chars.
	raw.Cflag |= syscall.CS8
	// Local flags: disable echo, canonical, signals, extended.
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG
	// Control chars: minimum 1 byte, no timeout.
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	if err := ioctlSetTermios(fd, &raw); err != nil {
		return fmt.Errorf("set raw mode: %w", err)
	}

	t.rawMode = true
	return nil
}

// exitRawMode restores the terminal to its original state.
func (t *TUI) exitRawMode() {
	if t.origTermios != nil {
		fd := os.Stdin.Fd()
		ioctlSetTermios(fd, t.origTermios)
		t.rawMode = false
	}
}

// Run enters raw mode and runs the render/input loop until the context is
// cancelled or Quit is called. It restores the terminal on exit.
func (t *TUI) Run(ctx context.Context) error {
	// Enter raw mode.
	if err := t.enterRawMode(); err != nil {
		return fmt.Errorf("enter raw mode: %w", err)
	}
	defer t.exitRawMode()

	// Hide cursor.
	fmt.Fprint(t.writer, CursorHide)
	defer fmt.Fprint(t.writer, CursorShow)

	// Get initial size.
	t.updateSize()

	// Handle SIGWINCH for terminal resize.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)

	// Start input reader.
	t.inputCh = make(chan []byte, 64)
	go t.readInput(ctx)

	// Initial render.
	t.Render()

	for {
		select {
		case <-ctx.Done():
			t.cleanup()
			return ctx.Err()

		case <-t.quit:
			t.cleanup()
			return nil

		case <-sigCh:
			t.mu.Lock()
			t.updateSize()
			t.lastLines = nil // Force full re-render on resize.
			t.mu.Unlock()
			t.Render()

		case data, ok := <-t.inputCh:
			if !ok {
				t.cleanup()
				return nil
			}
			t.handleInput(data)
			t.Render()
		}
	}
}

// Quit signals the TUI to exit the Run loop.
func (t *TUI) Quit() {
	select {
	case t.quit <- struct{}{}:
	default:
	}
}

// cleanup performs final cleanup before exiting.
func (t *TUI) cleanup() {
	t.mu.Lock()
	h := t.height
	t.mu.Unlock()
	fmt.Fprint(t.writer, CursorTo(h, 1))
	fmt.Fprint(t.writer, ClearToEndLine)
}

// readInput reads from stdin in a goroutine and sends input to inputCh.
func (t *TUI) readInput(ctx context.Context) {
	buf := make([]byte, 256)
	for {
		select {
		case <-ctx.Done():
			close(t.inputCh)
			return
		default:
		}

		n, err := os.Stdin.Read(buf)
		if err != nil {
			close(t.inputCh)
			return
		}
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			select {
			case t.inputCh <- data:
			case <-ctx.Done():
				close(t.inputCh)
				return
			}
		}
	}
}

// handleInput routes input to the focused component.
func (t *TUI) handleInput(data []byte) {
	t.mu.Lock()
	focus := t.focus
	onInput := t.onInput
	t.mu.Unlock()

	if focus != nil && focus.HandleInput(data) {
		return
	}

	if onInput != nil {
		onInput(data)
	}
}

// Render renders the root component and overlays, applying differential
// updates to minimize terminal output.
func (t *TUI) Render() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.root == nil {
		return
	}

	width := t.width
	height := t.height
	if width <= 0 || height <= 0 {
		return
	}

	// Render root component.
	lines := t.root.Render(width)

	// Apply overlays.
	if len(t.overlays) > 0 {
		lines = t.applyOverlays(lines, width, height)
	}

	// Truncate to terminal height.
	if len(lines) > height {
		lines = lines[:height]
	}

	// Pad to fill screen.
	for len(lines) < height {
		lines = append(lines, "")
	}

	// Differential rendering: find first changed line.
	firstDiff := 0
	if t.lastLines != nil && len(t.lastLines) == len(lines) {
		firstDiff = len(lines) // Assume no changes.
		for i := range lines {
			if i >= len(t.lastLines) || lines[i] != t.lastLines[i] {
				firstDiff = i
				break
			}
		}
	}

	if firstDiff >= len(lines) {
		// No changes.
		return
	}

	// Build output with synchronized output wrapping.
	var buf strings.Builder
	buf.WriteString(SyncStart)

	for i := firstDiff; i < len(lines); i++ {
		buf.WriteString(CursorTo(i+1, 1))
		buf.WriteString(ClearLine)

		line := lines[i]
		if VisibleWidth(line) > width {
			line = TruncateToWidth(line, width)
		}
		buf.WriteString(line)
	}

	buf.WriteString(SyncEnd)

	fmt.Fprint(t.writer, buf.String())

	// Store for next differential comparison.
	t.lastLines = make([]string, len(lines))
	copy(t.lastLines, lines)

	if t.onRender != nil {
		t.onRender()
	}
}

// applyOverlays composites overlay content on top of the base lines.
func (t *TUI) applyOverlays(base []string, width, height int) []string {
	result := make([]string, len(base))
	copy(result, base)

	for _, ov := range t.overlays {
		ovWidth := ov.options.Width
		if ovWidth <= 0 {
			ovWidth = width
		}
		if ovWidth > width {
			ovWidth = width
		}

		ovLines := ov.component.Render(ovWidth)

		ovHeight := len(ovLines)
		if ov.options.Height > 0 && ovHeight > ov.options.Height {
			ovHeight = ov.options.Height
			ovLines = ovLines[:ovHeight]
		}

		// Calculate vertical position.
		startRow := 0
		switch ov.options.Position {
		case OverlayCenter:
			startRow = (height - ovHeight) / 2
		case OverlayTop:
			startRow = 0
		case OverlayBottom:
			startRow = height - ovHeight
		}
		if startRow < 0 {
			startRow = 0
		}

		// Calculate horizontal offset for centering.
		hOffset := 0
		if ovWidth < width {
			hOffset = (width - ovWidth) / 2
		}

		// Composite overlay lines onto result.
		for i, ovLine := range ovLines {
			row := startRow + i
			if row >= len(result) {
				break
			}
			if hOffset > 0 {
				baseLine := result[row]
				baseVW := VisibleWidth(baseLine)
				left := ""
				if baseVW > 0 {
					left = TruncateToWidth(baseLine, hOffset)
				}
				leftVW := VisibleWidth(left)
				if leftVW < hOffset {
					left += strings.Repeat(" ", hOffset-leftVW)
				}
				result[row] = left + ovLine
			} else {
				result[row] = ovLine
			}
		}
	}

	return result
}

// ForceRender clears the render cache and forces a full re-render.
func (t *TUI) ForceRender() {
	t.mu.Lock()
	t.lastLines = nil
	t.mu.Unlock()
	t.root.Invalidate()
	t.Render()
}

// RenderToLines renders the root component to lines without outputting to
// the terminal. Useful for testing.
func (t *TUI) RenderToLines(width int) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.root == nil {
		return nil
	}
	return t.root.Render(width)
}
