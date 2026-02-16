package tui

import "sync"

// Component is the base interface for all TUI elements. Components produce
// lines of styled text that fit within the given width.
type Component interface {
	// Render returns the lines of output for this component, each fitting
	// within the specified terminal width (in columns).
	Render(width int) []string

	// Invalidate marks this component as needing re-render, clearing any
	// cached output.
	Invalidate()
}

// Focusable extends Component with focus and input handling capabilities.
// Components that accept keyboard input should implement this interface.
type Focusable interface {
	Component

	// SetFocused sets the focus state of this component.
	SetFocused(focused bool)

	// IsFocused returns whether this component currently has focus.
	IsFocused() bool

	// HandleInput processes raw terminal input. Returns true if the input
	// was consumed by this component, false if it should be passed to the
	// next handler.
	HandleInput(data []byte) bool
}

// BaseComponent provides a reusable foundation for Component implementations.
// It manages a dirty flag and cached render output so that subclasses only
// re-render when their state changes.
type BaseComponent struct {
	mu          sync.Mutex
	dirty       bool
	cachedWidth int
	cachedLines []string
}

// MarkDirty sets the dirty flag, indicating that the next Render call should
// recompute the output.
func (b *BaseComponent) MarkDirty() {
	b.mu.Lock()
	b.dirty = true
	b.mu.Unlock()
}

// Invalidate clears the render cache so the component will re-render on the
// next call to Render.
func (b *BaseComponent) Invalidate() {
	b.MarkDirty()
}

// IsDirty returns true if the component needs re-rendering.
func (b *BaseComponent) IsDirty() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dirty
}

// GetCached returns the cached lines if the component is not dirty and the
// width has not changed. Returns nil if re-rendering is needed.
func (b *BaseComponent) GetCached(width int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.dirty && b.cachedWidth == width && b.cachedLines != nil {
		cp := make([]string, len(b.cachedLines))
		copy(cp, b.cachedLines)
		return cp
	}
	return nil
}

// SetCached stores the rendered output in the cache.
func (b *BaseComponent) SetCached(width int, lines []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cachedWidth = width
	b.cachedLines = make([]string, len(lines))
	copy(b.cachedLines, lines)
	b.dirty = false
}

// Container is a component that holds and vertically stacks child components.
// Children are rendered in order, and their output lines are concatenated.
type Container struct {
	BaseComponent
	children []Component
}

// NewContainer creates a new Container with the given child components.
func NewContainer(children ...Component) *Container {
	return &Container{
		BaseComponent: BaseComponent{dirty: true},
		children:      children,
	}
}

// AddChild appends a child component to this container.
func (c *Container) AddChild(child Component) {
	c.children = append(c.children, child)
	c.MarkDirty()
}

// RemoveChild removes the first occurrence of the given child component.
func (c *Container) RemoveChild(child Component) {
	for i, ch := range c.children {
		if ch == child {
			c.children = append(c.children[:i], c.children[i+1:]...)
			c.MarkDirty()
			return
		}
	}
}

// Children returns the list of child components.
func (c *Container) Children() []Component {
	return c.children
}

// SetChildren replaces all children with the given list.
func (c *Container) SetChildren(children []Component) {
	c.children = children
	c.MarkDirty()
}

// Render produces the vertically concatenated output of all children.
func (c *Container) Render(width int) []string {
	if cached := c.GetCached(width); cached != nil {
		return cached
	}

	var lines []string
	for _, child := range c.children {
		childLines := child.Render(width)
		lines = append(lines, childLines...)
	}

	c.SetCached(width, lines)
	return lines
}

// Invalidate marks this container and all its children as dirty.
func (c *Container) Invalidate() {
	c.MarkDirty()
	for _, child := range c.children {
		child.Invalidate()
	}
}

// FindFocusable searches this container's children (depth-first) for a
// Focusable component that currently has focus. Returns nil if none is focused.
func (c *Container) FindFocusable() Focusable {
	for _, child := range c.children {
		if f, ok := child.(Focusable); ok && f.IsFocused() {
			return f
		}
		if sub, ok := child.(*Container); ok {
			if f := sub.FindFocusable(); f != nil {
				return f
			}
		}
	}
	return nil
}

// AllFocusables returns all Focusable components in this container tree
// in depth-first order.
func (c *Container) AllFocusables() []Focusable {
	var result []Focusable
	for _, child := range c.children {
		if f, ok := child.(Focusable); ok {
			result = append(result, f)
		}
		if sub, ok := child.(*Container); ok {
			result = append(result, sub.AllFocusables()...)
		}
	}
	return result
}
