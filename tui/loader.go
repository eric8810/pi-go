package tui

import (
	"sync"
	"time"
)

// DefaultSpinnerFrames are braille dot patterns used for the default spinner animation.
var DefaultSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// DotsSpinnerFrames provides an alternative dots animation.
var DotsSpinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// LineSpinnerFrames provides a simple line-based spinner.
var LineSpinnerFrames = []string{"|", "/", "-", "\\"}

// Loader is an animated spinner component that displays a spinning indicator
// alongside a label. It uses a ticker to drive animation and calls Invalidate
// on each frame change.
type Loader struct {
	BaseComponent
	mu       sync.Mutex
	label    string
	frames   []string
	frame    int
	interval time.Duration
	ticker   *time.Ticker
	stopCh   chan struct{}
	running  bool
	style    string // ANSI style for the spinner character.
	onTick   func() // Called on each tick, can be used to trigger TUI re-render.
}

// NewLoader creates a new Loader with the given label and default braille
// spinner frames. The loader does not start animating until Start is called.
func NewLoader(label string) *Loader {
	return &Loader{
		BaseComponent: BaseComponent{dirty: true},
		label:         label,
		frames:        DefaultSpinnerFrames,
		interval:      80 * time.Millisecond,
		style:         FgCyan,
	}
}

// SetLabel updates the label text displayed next to the spinner.
func (l *Loader) SetLabel(label string) {
	l.mu.Lock()
	l.label = label
	l.mu.Unlock()
	l.MarkDirty()
}

// Label returns the current label text.
func (l *Loader) Label() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.label
}

// SetFrames sets the animation frames for the spinner.
func (l *Loader) SetFrames(frames []string) {
	l.mu.Lock()
	l.frames = frames
	l.frame = 0
	l.mu.Unlock()
	l.MarkDirty()
}

// SetInterval sets the time between animation frames.
func (l *Loader) SetInterval(d time.Duration) {
	l.mu.Lock()
	l.interval = d
	l.mu.Unlock()
}

// SetStyle sets the ANSI style applied to the spinner character.
func (l *Loader) SetStyle(style string) {
	l.mu.Lock()
	l.style = style
	l.mu.Unlock()
	l.MarkDirty()
}

// SetOnTick sets a callback that is invoked on each animation tick. This is
// typically used to trigger a TUI re-render.
func (l *Loader) SetOnTick(fn func()) {
	l.mu.Lock()
	l.onTick = fn
	l.mu.Unlock()
}

// Start begins the spinner animation. It is safe to call Start multiple
// times; subsequent calls are no-ops if the loader is already running.
func (l *Loader) Start() {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return
	}
	l.running = true
	l.stopCh = make(chan struct{})
	l.ticker = time.NewTicker(l.interval)
	l.mu.Unlock()

	go l.animate()
}

// Stop halts the spinner animation.
func (l *Loader) Stop() {
	l.mu.Lock()
	if !l.running {
		l.mu.Unlock()
		return
	}
	l.running = false
	close(l.stopCh)
	l.ticker.Stop()
	l.mu.Unlock()
}

// IsRunning returns whether the spinner animation is currently active.
func (l *Loader) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

// animate runs the animation loop in a goroutine.
func (l *Loader) animate() {
	for {
		select {
		case <-l.stopCh:
			return
		case <-l.ticker.C:
			l.mu.Lock()
			l.frame = (l.frame + 1) % len(l.frames)
			onTick := l.onTick
			l.mu.Unlock()
			l.MarkDirty()
			if onTick != nil {
				onTick()
			}
		}
	}
}

// Render produces a single line with the spinner and label.
func (l *Loader) Render(width int) []string {
	l.mu.Lock()
	frame := l.frames[l.frame%len(l.frames)]
	label := l.label
	style := l.style
	l.mu.Unlock()

	spinner := style + frame + Reset
	line := spinner + " " + label

	if VisibleWidth(line) > width {
		line = TruncateToWidth(line, width)
	}

	return []string{line}
}

// CancellableLoader wraps a Loader and adds a key hint for cancellation.
type CancellableLoader struct {
	BaseComponent
	loader    *Loader
	cancelKey string // Display string for the cancel key (e.g., "Esc").
	onCancel  func()
	focused   bool
}

// NewCancellableLoader creates a new CancellableLoader with a spinner, label,
// and a cancel key hint.
func NewCancellableLoader(label string, cancelKey string) *CancellableLoader {
	return &CancellableLoader{
		BaseComponent: BaseComponent{dirty: true},
		loader:        NewLoader(label),
		cancelKey:     cancelKey,
	}
}

// Loader returns the underlying Loader component.
func (cl *CancellableLoader) Loader() *Loader {
	return cl.loader
}

// SetOnCancel sets the callback invoked when the cancel key is pressed.
func (cl *CancellableLoader) SetOnCancel(fn func()) {
	cl.onCancel = fn
}

// SetFocused sets the focus state.
func (cl *CancellableLoader) SetFocused(focused bool) {
	cl.focused = focused
	cl.MarkDirty()
}

// IsFocused returns whether this component has focus.
func (cl *CancellableLoader) IsFocused() bool {
	return cl.focused
}

// Start begins the spinner animation.
func (cl *CancellableLoader) Start() {
	cl.loader.Start()
}

// Stop halts the spinner animation.
func (cl *CancellableLoader) Stop() {
	cl.loader.Stop()
}

// Render produces a line with the spinner, label, and cancel hint.
func (cl *CancellableLoader) Render(width int) []string {
	loaderLines := cl.loader.Render(width)
	if len(loaderLines) == 0 {
		return []string{""}
	}

	hint := DimOn + " (" + cl.cancelKey + " to cancel)" + DimOff
	line := loaderLines[0] + hint

	if VisibleWidth(line) > width {
		line = TruncateToWidth(line, width)
	}

	return []string{line}
}

// HandleInput processes keyboard input for the cancellable loader.
func (cl *CancellableLoader) HandleInput(data []byte) bool {
	// Check for escape key as the default cancel key.
	if MatchesKey(data, KeyEscape) {
		if cl.onCancel != nil {
			cl.onCancel()
		}
		return true
	}
	// Also check ctrl+c.
	if MatchesKey(data, KeyCtrlC) {
		if cl.onCancel != nil {
			cl.onCancel()
		}
		return true
	}
	return false
}

// Invalidate marks both the wrapper and the inner loader as dirty.
func (cl *CancellableLoader) Invalidate() {
	cl.MarkDirty()
	cl.loader.Invalidate()
}
