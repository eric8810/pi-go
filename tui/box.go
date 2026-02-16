package tui

import "strings"

// BorderStyle defines the characters used to draw a box border.
type BorderStyle struct {
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	Horizontal  string
	Vertical    string
}

// Predefined border styles.
var (
	// BorderNone draws no border.
	BorderNone = BorderStyle{}

	// BorderSingle uses single-line box-drawing characters.
	BorderSingle = BorderStyle{
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
		Horizontal:  "─",
		Vertical:    "│",
	}

	// BorderDouble uses double-line box-drawing characters.
	BorderDouble = BorderStyle{
		TopLeft:     "╔",
		TopRight:    "╗",
		BottomLeft:  "╚",
		BottomRight: "╝",
		Horizontal:  "═",
		Vertical:    "║",
	}

	// BorderRounded uses rounded corners with single-line sides.
	BorderRounded = BorderStyle{
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
		Horizontal:  "─",
		Vertical:    "│",
	}

	// BorderHeavy uses heavy/thick box-drawing characters.
	BorderHeavy = BorderStyle{
		TopLeft:     "┏",
		TopRight:    "┓",
		BottomLeft:  "┗",
		BottomRight: "┛",
		Horizontal:  "━",
		Vertical:    "┃",
	}
)

// hasBorder returns true if this border style draws any visible border.
func (bs BorderStyle) hasBorder() bool {
	return bs.Horizontal != "" || bs.Vertical != ""
}

// Padding specifies spacing around a component in rows (top/bottom) and
// columns (left/right).
type Padding struct {
	Top    int
	Right  int
	Bottom int
	Left   int
}

// UniformPadding creates a Padding with the same value on all sides.
func UniformPadding(n int) Padding {
	return Padding{Top: n, Right: n, Bottom: n, Left: n}
}

// HVPadding creates a Padding with separate horizontal and vertical values.
func HVPadding(horizontal, vertical int) Padding {
	return Padding{Top: vertical, Right: horizontal, Bottom: vertical, Left: horizontal}
}

// Box is a component that wraps a child component with padding, an optional
// border, and an optional background color.
type Box struct {
	BaseComponent
	child      Component
	padding    Padding
	border     BorderStyle
	background string // ANSI background escape code.
	borderColor string // ANSI color for border characters.
}

// NewBox creates a new Box wrapping the given child component.
func NewBox(child Component) *Box {
	return &Box{
		BaseComponent: BaseComponent{dirty: true},
		child:         child,
	}
}

// SetChild sets the wrapped child component.
func (b *Box) SetChild(child Component) {
	b.child = child
	b.MarkDirty()
}

// Child returns the wrapped child component.
func (b *Box) Child() Component {
	return b.child
}

// SetPadding sets the padding around the child.
func (b *Box) SetPadding(p Padding) {
	b.padding = p
	b.MarkDirty()
}

// SetBorder sets the border style.
func (b *Box) SetBorder(style BorderStyle) {
	b.border = style
	b.MarkDirty()
}

// SetBackground sets the background color as an ANSI escape code.
func (b *Box) SetBackground(bg string) {
	b.background = bg
	b.MarkDirty()
}

// SetBorderColor sets the ANSI color code for the border characters.
func (b *Box) SetBorderColor(color string) {
	b.borderColor = color
	b.MarkDirty()
}

// Render produces lines for the box, including border, padding, and child content.
func (b *Box) Render(width int) []string {
	if cached := b.GetCached(width); cached != nil {
		return cached
	}

	hasBorder := b.border.hasBorder()

	// Calculate interior width.
	borderW := 0
	if hasBorder {
		borderW = 2 // left + right border characters
	}
	innerWidth := width - borderW - b.padding.Left - b.padding.Right
	if innerWidth < 1 {
		innerWidth = 1
	}

	// Render child.
	var childLines []string
	if b.child != nil {
		childLines = b.child.Render(innerWidth)
	}

	var lines []string

	// Top border.
	if hasBorder {
		topLine := b.borderStr(b.border.TopLeft) +
			strings.Repeat(b.border.Horizontal, width-borderW) +
			b.borderStr(b.border.TopRight)
		if b.borderColor != "" {
			topLine = b.borderColor + topLine + Reset
		}
		lines = append(lines, topLine)
	}

	// Top padding.
	for i := 0; i < b.padding.Top; i++ {
		lines = append(lines, b.makeEmptyLine(width, hasBorder))
	}

	// Child content lines.
	for _, cl := range childLines {
		lines = append(lines, b.makeContentLine(cl, width, innerWidth, hasBorder))
	}

	// Bottom padding.
	for i := 0; i < b.padding.Bottom; i++ {
		lines = append(lines, b.makeEmptyLine(width, hasBorder))
	}

	// Bottom border.
	if hasBorder {
		bottomLine := b.borderStr(b.border.BottomLeft) +
			strings.Repeat(b.border.Horizontal, width-borderW) +
			b.borderStr(b.border.BottomRight)
		if b.borderColor != "" {
			bottomLine = b.borderColor + bottomLine + Reset
		}
		lines = append(lines, bottomLine)
	}

	b.SetCached(width, lines)
	return lines
}

// borderStr wraps a border character with the border color if set.
func (b *Box) borderStr(ch string) string {
	return ch
}

// makeEmptyLine creates a line with just border and padding (no content).
func (b *Box) makeEmptyLine(width int, hasBorder bool) string {
	var buf strings.Builder

	if b.background != "" {
		buf.WriteString(b.background)
	}

	if hasBorder {
		if b.borderColor != "" {
			buf.WriteString(b.borderColor)
		}
		buf.WriteString(b.border.Vertical)
		if b.borderColor != "" {
			buf.WriteString(Reset)
			if b.background != "" {
				buf.WriteString(b.background)
			}
		}
	}

	fillWidth := width
	if hasBorder {
		fillWidth -= 2
	}
	if fillWidth > 0 {
		buf.WriteString(strings.Repeat(" ", fillWidth))
	}

	if hasBorder {
		if b.borderColor != "" {
			buf.WriteString(b.borderColor)
		}
		buf.WriteString(b.border.Vertical)
	}

	if b.background != "" || b.borderColor != "" {
		buf.WriteString(Reset)
	}

	return buf.String()
}

// makeContentLine creates a line with border, padding, and content.
func (b *Box) makeContentLine(content string, totalWidth, innerWidth int, hasBorder bool) string {
	var buf strings.Builder

	if b.background != "" {
		buf.WriteString(b.background)
	}

	if hasBorder {
		if b.borderColor != "" {
			buf.WriteString(b.borderColor)
		}
		buf.WriteString(b.border.Vertical)
		if b.borderColor != "" {
			buf.WriteString(Reset)
			if b.background != "" {
				buf.WriteString(b.background)
			}
		}
	}

	// Left padding.
	if b.padding.Left > 0 {
		buf.WriteString(strings.Repeat(" ", b.padding.Left))
	}

	// Content, padded to fill innerWidth.
	contentWidth := VisibleWidth(content)
	buf.WriteString(content)
	if contentWidth < innerWidth {
		buf.WriteString(strings.Repeat(" ", innerWidth-contentWidth))
	}

	// Right padding.
	if b.padding.Right > 0 {
		buf.WriteString(strings.Repeat(" ", b.padding.Right))
	}

	if hasBorder {
		if b.borderColor != "" {
			buf.WriteString(b.borderColor)
		}
		buf.WriteString(b.border.Vertical)
	}

	if b.background != "" || b.borderColor != "" {
		buf.WriteString(Reset)
	}

	return buf.String()
}

// Invalidate marks the box and its child as dirty.
func (b *Box) Invalidate() {
	b.MarkDirty()
	if b.child != nil {
		b.child.Invalidate()
	}
}
