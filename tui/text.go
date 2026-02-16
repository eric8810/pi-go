package tui

import "strings"

// Alignment specifies how text is aligned within the available width.
type Alignment int

const (
	// AlignLeft aligns text to the left (default).
	AlignLeft Alignment = iota
	// AlignCenter centers text within the available width.
	AlignCenter
	// AlignRight aligns text to the right.
	AlignRight
)

// TextStyle defines the visual style applied to a Text component.
type TextStyle struct {
	Bold      bool
	Dim       bool
	Italic    bool
	Underline bool
	Color     string // ANSI color escape code to prepend (e.g., FgRed).
}

// apply wraps the given string with the style's ANSI codes.
func (ts TextStyle) apply(s string) string {
	if ts.Color == "" && !ts.Bold && !ts.Dim && !ts.Italic && !ts.Underline {
		return s
	}
	var prefix strings.Builder
	if ts.Color != "" {
		prefix.WriteString(ts.Color)
	}
	if ts.Bold {
		prefix.WriteString(BoldOn)
	}
	if ts.Dim {
		prefix.WriteString(DimOn)
	}
	if ts.Italic {
		prefix.WriteString(ItalicOn)
	}
	if ts.Underline {
		prefix.WriteString(UnderlineOn)
	}
	return prefix.String() + s + Reset
}

// Text is a component that renders multi-line wrapped text with optional
// styling and alignment.
type Text struct {
	BaseComponent
	content   string
	style     TextStyle
	alignment Alignment
	maxLines  int // 0 means unlimited.
}

// NewText creates a new Text component with the given content.
func NewText(content string) *Text {
	return &Text{
		BaseComponent: BaseComponent{dirty: true},
		content:       content,
	}
}

// SetContent updates the text content and marks the component as dirty.
func (t *Text) SetContent(content string) {
	t.content = content
	t.MarkDirty()
}

// Content returns the current text content.
func (t *Text) Content() string {
	return t.content
}

// SetStyle sets the text style.
func (t *Text) SetStyle(style TextStyle) {
	t.style = style
	t.MarkDirty()
}

// SetAlignment sets the text alignment.
func (t *Text) SetAlignment(alignment Alignment) {
	t.alignment = alignment
	t.MarkDirty()
}

// SetMaxLines sets the maximum number of lines to render. Zero means unlimited.
func (t *Text) SetMaxLines(n int) {
	t.maxLines = n
	t.MarkDirty()
}

// Render produces wrapped, styled, and aligned lines of text.
func (t *Text) Render(width int) []string {
	if cached := t.GetCached(width); cached != nil {
		return cached
	}

	if t.content == "" {
		lines := []string{""}
		t.SetCached(width, lines)
		return lines
	}

	wrapped := WrapText(t.content, width)

	if t.maxLines > 0 && len(wrapped) > t.maxLines {
		wrapped = wrapped[:t.maxLines]
	}

	lines := make([]string, len(wrapped))
	for i, line := range wrapped {
		styled := t.style.apply(line)
		lines[i] = t.align(styled, width)
	}

	t.SetCached(width, lines)
	return lines
}

// align applies the configured alignment to a single line.
func (t *Text) align(line string, width int) string {
	switch t.alignment {
	case AlignCenter:
		return PadCenter(line, width)
	case AlignRight:
		vw := VisibleWidth(line)
		if vw >= width {
			return line
		}
		return strings.Repeat(" ", width-vw) + line
	default:
		return line
	}
}

// TruncatedText is a component that renders a single line of text, truncated
// with an ellipsis if it exceeds the available width.
type TruncatedText struct {
	BaseComponent
	content string
	style   TextStyle
}

// NewTruncatedText creates a new TruncatedText component.
func NewTruncatedText(content string) *TruncatedText {
	return &TruncatedText{
		BaseComponent: BaseComponent{dirty: true},
		content:       content,
	}
}

// SetContent updates the text content.
func (t *TruncatedText) SetContent(content string) {
	t.content = content
	t.MarkDirty()
}

// Content returns the current text content.
func (t *TruncatedText) Content() string {
	return t.content
}

// SetStyle sets the text style.
func (t *TruncatedText) SetStyle(style TextStyle) {
	t.style = style
	t.MarkDirty()
}

// Render produces a single truncated line of styled text.
func (t *TruncatedText) Render(width int) []string {
	if cached := t.GetCached(width); cached != nil {
		return cached
	}

	line := t.style.apply(t.content)
	if VisibleWidth(line) > width {
		line = TruncateToWidth(t.style.apply(t.content), width)
	}

	lines := []string{line}
	t.SetCached(width, lines)
	return lines
}

// Spacer is a component that renders a specified number of empty lines.
type Spacer struct {
	BaseComponent
	lines int
}

// NewSpacer creates a new Spacer that renders n empty lines.
func NewSpacer(n int) *Spacer {
	if n < 1 {
		n = 1
	}
	return &Spacer{
		BaseComponent: BaseComponent{dirty: true},
		lines:         n,
	}
}

// SetLines sets the number of empty lines.
func (s *Spacer) SetLines(n int) {
	if n < 1 {
		n = 1
	}
	s.lines = n
	s.MarkDirty()
}

// Render produces the specified number of empty lines.
func (s *Spacer) Render(width int) []string {
	if cached := s.GetCached(width); cached != nil {
		return cached
	}

	lines := make([]string, s.lines)
	for i := range lines {
		lines[i] = ""
	}

	s.SetCached(width, lines)
	return lines
}
