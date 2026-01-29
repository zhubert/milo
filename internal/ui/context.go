package ui

// ViewContext holds shared layout state.
type ViewContext struct {
	TerminalWidth  int
	TerminalHeight int
	ContentHeight  int
}

var viewContext = &ViewContext{
	TerminalWidth:  80,
	TerminalHeight: 24,
	ContentHeight:  20,
}

// GetViewContext returns the shared view context.
func GetViewContext() *ViewContext {
	return viewContext
}

// UpdateTerminalSize updates the context with new terminal dimensions.
func (c *ViewContext) UpdateTerminalSize(width, height int) {
	c.TerminalWidth = width
	c.TerminalHeight = height
	// Reserve space for header (0 lines now) + footer (1 line) + input area (3 lines).
	c.ContentHeight = height - 4
	if c.ContentHeight < 1 {
		c.ContentHeight = 1
	}
}
